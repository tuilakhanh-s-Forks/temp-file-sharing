package pkg

import (
	"fmt"
	"hash/fnv"
	"math/rand"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog/log"

	"github.com/tuilakhanh/webshare/internal/config"
)

func RandomName(seedString string) string {
	h := fnv.New32a()
	h.Write([]byte(seedString))
	seed := int64(h.Sum32())
	src := rand.New(rand.NewSource(seed))
	return fmt.Sprintf("%d%d%d", src.Intn(10), src.Intn(10), src.Intn(10))
}

// TrimContent will continually purge things from the content directory until
// the content directoyr is below the specified size
func TrimContent(config config.Config) {
	i := 0
	for {
		i++
		if i > 30 {
			// avoid the infinite loop
			break
		}
		dirSize, largestFile, err := DirSize(config.ContentDirectory)
		if err != nil {
			log.Error().Err(err).Msg("Error getting directory size")
		}
		if dirSize < config.MaxBytesTotal || largestFile.Id == "" {
			break
		}
		log.Debug().
			Int64("dir_size", dirSize).
			Int64("max_bytes_total", config.MaxBytesTotal).
			Msg("Bytes in directory exceed maximum")

		log.Debug().
			Str("file_id", largestFile.Id).
			Msg("Removing file")

		os.RemoveAll(path.Join(config.ContentDirectory, largestFile.Id))
	}
}

// DirSize returns the size of a directory in bytes
func DirSize(path string) (int64, *fileInfo, error) {
	var totalSize int64
	var largestFile *fileInfo

	err := filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			totalSize += info.Size()

			if largestFile == nil || info.Size() > largestFile.Size {
				id := strings.Split(filePath, "/")[1]
				largestFile = &fileInfo{Id: id, Path: filePath, Size: info.Size()}
			}
		}
		return err
	})
	return totalSize, largestFile, err
}

type fileInfo struct {
	Id   string
	Path string
	Size int64
}
