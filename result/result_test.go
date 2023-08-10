package result

import (
	"bytes"
	"fmt"
	"math/rand"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

var letterRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

func RandStringRunes(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}

func TestChunkFile(t *testing.T) {
	for _, length := range []int{
		0,
		1,
		10,
		1024 * 1024,
		1024*1024 + 1,
		10 * 1024 * 1024,
		10*1024*1024 + 20,
	} {
		t.Run(fmt.Sprintf("%d", length), func(t *testing.T) {
			content := RandStringRunes(length)
			tempfile, err := os.CreateTemp("", "")
			require.NoError(t, err)
			defer func() {
				require.NoError(t, os.Remove(tempfile.Name()))
			}()
			_, err = tempfile.WriteString(content)
			require.NoError(t, err)
			require.NoError(t, tempfile.Close())

			buf := bytes.Buffer{}
			require.NoError(t, chunkFile(tempfile.Name(), func(b []byte) error {
				_, err := buf.Write(b)
				return err
			}))
			require.Equal(t, content, buf.String())

			buf = bytes.Buffer{}
			require.NoError(t, chunkByte([]byte(content), func(b []byte) error {
				_, err := buf.Write(b)
				return err
			}))
			require.Equal(t, content, buf.String())
		})
	}
}
