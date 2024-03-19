package result

import (
	"bufio"
	"bytes"
	"context"
	go_errors "errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"time"

	"github.com/pkg/errors"
	"github.com/prodvana/prodvana-public/go/prodvana-sdk/client"
	blobs_pb "github.com/prodvana/prodvana-public/go/prodvana-sdk/proto/prodvana/blobs"
	pvn_wrapper_pb "github.com/prodvana/prodvana-public/go/prodvana-sdk/proto/prodvana/pvn_wrapper"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/encoding/protojson"
)

type OutputFileUpload struct {
	Name   string
	Stdout bool
	Stderr bool

	// only one or the other can be specified
	Path    string
	Content []byte
}

type InputFile struct {
	Path   string
	BlobId string
}

const (
	PvnWrapperVersion = "0.0.2"
)

func chunkReader(reader io.Reader, process func([]byte) error) error {
	reader = bufio.NewReader(reader)
	const chunkSize = 1024 * 1024
	buf := make([]byte, chunkSize)
	sentOnce := false
	for {
		n, err := reader.Read(buf)
		if err != nil {
			if go_errors.Is(err, io.EOF) {
				break
			}
			return err
		}
		sentOnce = true
		err = process(buf[:n])
		if err != nil {
			return err
		}
	}
	if !sentOnce {
		// HACK(naphat) somewhere our handling of empty files is incorrect, requiring us to send a no-op upload req once to avoid hanging
		err := process(nil)
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

func downloadBlob(ctx context.Context, blobsClient blobs_pb.BlobsManagerClient, file InputFile) error {
	strm, err := blobsClient.GetCasBlob(ctx, &blobs_pb.GetCasBlobReq{
		Id: file.BlobId,
	})
	if err != nil {
		return errors.Wrapf(err, "failed to initiate download of blob %s", file.BlobId)
	}
	defer func() { _ = strm.CloseSend() }()
	f, err := os.Create(file.Path)
	if err != nil {
		return errors.Wrapf(err, "failed to open %s", file.Path)
	}
	defer func() { _ = f.Close() }()
	for {
		resp, err := strm.Recv()
		if err != nil {
			if go_errors.Is(err, io.EOF) {
				break
			}
			return errors.Wrapf(err, "failed to download blob %s", file.BlobId)
		}
		_, err = f.Write(resp.Bytes)
		if err != nil {
			return errors.Wrapf(err, "failed to write to %s", file.Path)
		}
	}
	return nil
}

// Handle the "main" function of wrapper commands.
// This function never returns.
func RunWrapper(inputFiles []InputFile, successExitCodes []int32, run func(context.Context) (*pvn_wrapper_pb.Output, []OutputFileUpload, error)) {
	ctx := context.Background()
	var conn *grpc.ClientConn
	getProdvanaConnection := func() *grpc.ClientConn {
		var err error
		conn, err = client.MakeProdvanaConnection(client.DefaultConnectionOptions())
		if err != nil {
			// TODO(naphat) should we return json in the event of infra errors too?
			log.Fatal(err)
		}
		return conn
	}
	var blobsClient blobs_pb.BlobsManagerClient
	getBlobsClient := func() blobs_pb.BlobsManagerClient {
		if blobsClient == nil {
			blobsClient = blobs_pb.NewBlobsManagerClient(getProdvanaConnection())
		}
		return blobsClient
	}
	var jobClient pvn_wrapper_pb.JobManagerClient
	getJobClient := func() pvn_wrapper_pb.JobManagerClient {
		if jobClient == nil {
			jobClient = pvn_wrapper_pb.NewJobManagerClient(getProdvanaConnection())
		}
		return jobClient
	}
	defer func() {
		if conn != nil {
			_ = conn.Close()
		}
	}()
	for _, input := range inputFiles {
		if err := downloadBlob(ctx, getBlobsClient(), input); err != nil {
			log.Fatal(err)
		}
	}
	startTs := time.Now()
	result, outputFiles, err := run(ctx)
	duration := time.Since(startTs)
	if err != nil {
		result := &pvn_wrapper_pb.Output{}
		result.ExecError = err.Error()
		result.ExitCode = -1
	}
	hostname, err := os.Hostname()
	if err == nil {
		result.Hostname = hostname
	}
	result.StartTimestampNs = startTs.UnixNano()
	result.DurationNs = duration.Nanoseconds()
	result.Version = PvnWrapperVersion
	isSuccessful := false
	for _, exitCode := range successExitCodes {
		if exitCode == result.ExitCode {
			isSuccessful = true
			break
		}
	}
	if len(outputFiles) > 0 {
		defer func() { _ = conn.Close() }()
		for _, file := range outputFiles {
			id, uploadErr := uploadOutput(ctx, getBlobsClient(), file)
			if uploadErr != nil {
				if !os.IsNotExist(uploadErr) || isSuccessful {
					// for IsNotExist errors in the event the program did not exit successfully, do not hard error on missing output file.
					// TODO(naphat) should we return json in the event of infra errors too?
					// for now, print out every output file so that we have the output for debugging
					for _, file := range outputFiles {
						if file.Stderr {
							fmt.Printf("Stderr:\n")
						} else if file.Stdout {
							fmt.Printf("Stdout:\n")
						} else {
							fmt.Printf("Output file: %s\n", file.Name)
						}
						var bytes []byte
						var err error
						if file.Path != "" {
							bytes, err = os.ReadFile(file.Path)
							if err != nil {
								log.Printf("Failed to read file %s: %v", file.Path, err)
							}
						} else {
							bytes = file.Content
						}
						_, err = os.Stdout.Write(bytes)
						if err != nil {
							log.Printf("Failed to write output %s for debugging: %+v", file.Name, err)
						}
					}
					fileName := file.Path
					if file.Stderr {
						fileName = "stderr"
					}
					if file.Stdout {
						fileName = "stdout"
					}
					log.Fatalf("failed to upload file %s: %+v\n", fileName, uploadErr)
				}
				continue
			}
			if file.Stdout {
				if result.StdoutBlobId != "" {
					log.Fatal("internal error: multiple stdout provided")
				}
				result.StdoutBlobId = id
			} else if file.Stderr {
				if result.StderrBlobId != "" {
					log.Fatal("internal error: multiple stderr provided")
				}
				result.StderrBlobId = id
			} else {
				result.Files = append(result.Files, &pvn_wrapper_pb.OutputFile{
					Name:          file.Name,
					ContentBlobId: id,
				})
			}
		}
	}

	jobId := os.Getenv("PVN_JOB_ID")
	if jobId != "" {
		_, err := getJobClient().ReportJobResult(ctx, &pvn_wrapper_pb.ReportJobResultReq{
			JobId:  jobId,
			Output: result,
		})
		if err != nil {
			log.Fatal(err)
		}
	}

	output, err := protojson.Marshal(result)
	if err != nil {
		// If something went wrong during encode/write to stdout, indicate that in stderr and exit non-zero.
		log.Fatal(err)
	}
	_, err = os.Stdout.Write(output)
	if err != nil {
		log.Fatal(err)
	}

	// If the wrapped process fails, make sure this process has a non-zero exit code.
	// This is to maintain compatibility with existing task execution infrastructure.
	// Once we enforce the use of this wrapper, we can safely exit 0 here.
	os.Exit(int(result.ExitCode))
}

func RunCmd(cmd *exec.Cmd) (*pvn_wrapper_pb.Output, []OutputFileUpload, error) {
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	var result pvn_wrapper_pb.Output

	err := cmd.Run()

	if err != nil {
		var exitErr *exec.ExitError
		if go_errors.As(err, &exitErr) {
			result.ExitCode = int32(exitErr.ExitCode())
		} else {
			return nil, nil, err
		}
	}

	return &result, []OutputFileUpload{
		{
			Stdout:  true,
			Content: stdout.Bytes(),
		},
		{
			Stderr:  true,
			Content: stderr.Bytes(),
		},
	}, nil
}
