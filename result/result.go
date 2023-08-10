package result

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	go_errors "errors"
	"io"
	"log"
	"os"
	"os/exec"
	"time"

	"github.com/prodvana/prodvana-public/go/prodvana-sdk/client"
	blobs_pb "github.com/prodvana/prodvana-public/go/prodvana-sdk/proto/prodvana/blobs"
)

type ResultType struct {
	ExitCode         int    `json:"exit_code"`  // Exit code of wrapped process. -1 if process failed to execute.
	ExecError        string `json:"exec_error"` // Internal error when trying to execute wrapped process.
	Stdout           []byte `json:"stdout"`
	Stderr           []byte `json:"stderr"`
	Version          string `json:"version"`     // Wrapper version.
	StartTimestampNs int64  `json:"start_ts_ns"` // Timestamp when the process began executing, in ns.
	DurationNs       int64  `json:"duration_ns"` // Total execution duration of the process, in ns.
	Files            []File `json:"files"`
}

type File struct {
	Name          string `json:"name"`
	ContentBlobId string `json:"content_blob_id"`
}

type OutputFileUpload struct {
	Name string

	// only one or the other can be specified
	Path    string
	Content []byte
}

const (
	PvnWrapperVersion = "0.0.2"
)

func chunkReader(reader io.Reader, process func([]byte) error) error {
	reader = bufio.NewReader(reader)
	const chunkSize = 1024 * 1024
	buf := make([]byte, chunkSize)
	for {
		n, err := reader.Read(buf)
		if err != nil {
			if go_errors.Is(err, io.EOF) {
				break
			}
			return err
		}
		err = process(buf[:n])
		if err != nil {
			return err
		}
	}
	return nil
}

func chunkFile(path string, process func([]byte) error) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()
	return chunkReader(file, process)
}

func chunkByte(content []byte, process func([]byte) error) error {
	reader := bytes.NewReader(content)
	return chunkReader(reader, process)
}

func uploadOutput(ctx context.Context, blobsClient blobs_pb.BlobsManagerClient, file OutputFileUpload) (string, error) {
	strm, err := blobsClient.UploadCasBlob(ctx)
	if err != nil {
		return "", err
	}
	process := func(b []byte) error {
		return strm.Send(&blobs_pb.UploadCasBlobReq{
			Bytes: b,
		})
	}
	if file.Path != "" {
		err = chunkFile(file.Path, process)
	} else {
		err = chunkByte(file.Content, process)
	}
	if err != nil {
		return "", err
	}
	resp, err := strm.CloseAndRecv()
	if err != nil {
		return "", err
	}
	return resp.Id, nil
}

// Handle the "main" function of wrapper commands.
// This function never returns.
func RunWrapper(run func(context.Context) (*ResultType, []OutputFileUpload, error)) {
	ctx := context.Background()
	startTs := time.Now()
	result, outputFiles, err := run(ctx)
	duration := time.Since(startTs)
	if err != nil {
		result := &ResultType{}
		result.ExecError = err.Error()
		result.ExitCode = -1
	}
	result.StartTimestampNs = startTs.UnixNano()
	result.DurationNs = duration.Nanoseconds()
	result.Version = PvnWrapperVersion
	if len(outputFiles) > 0 {
		conn, err := client.MakeProdvanaConnection(client.DefaultConnectionOptions())
		if err != nil {
			// TODO(naphat) should we return json in the event of infra errors too?
			log.Fatal(err)
		}
		defer func() { _ = conn.Close() }()
		blobsClient := blobs_pb.NewBlobsManagerClient(conn)
		for _, file := range outputFiles {
			id, err := uploadOutput(ctx, blobsClient, file)
			if err != nil {
				// TODO(naphat) should we return json in the event of infra errors too?
				log.Fatal(err)
			}
			result.Files = append(result.Files, File{
				Name:          file.Name,
				ContentBlobId: id,
			})
		}
	}

	err = json.NewEncoder(os.Stdout).Encode(result)
	if err != nil {
		// If something went wrong during encode/write to stdout, indicate that in stderr and exit non-zero.
		log.Fatal(err)
	}

	// If the wrapped process fails, make sure this process has a non-zero exit code.
	// This is to maintain compatibility with existing task execution infrastructure.
	// Once we enforce the use of this wrapper, we can safely exit 0 here.
	os.Exit(result.ExitCode)
}

func RunCmd(cmd *exec.Cmd) (*ResultType, error) {
	// TODO: Limit stdout/stderr to a reasonable size while preserving useful error context.
	// Kubernetes output is usually limited to 10MB.
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	var result ResultType

	err := cmd.Run()

	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			result.ExitCode = exitErr.ExitCode()
		} else {
			return nil, err
		}
	}

	result.Stdout = stdout.Bytes()
	result.Stderr = stderr.Bytes()
	return &result, nil
}
