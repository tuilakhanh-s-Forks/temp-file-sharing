package handlers

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/rs/zerolog/log"

	"github.com/tuilakhanh/webshare/internal/config"
	"github.com/tuilakhanh/webshare/internal/pkg"
)

// TrimContent will continually purge things from the content directory until
// the content directoyr is below the specified size
func TrimContent(config config.Config) {
	for i := 0; i < 30; i++ {
		dirSize, biggestFileID, err := DirSize(config.ContentDirectory)
		if err != nil {
			log.Error().Err(err).Msg("Error getting directory size")
		}
		if dirSize < config.MaxBytesTotal || biggestFileID == "" {
			break
		}
		log.Debug().
			Int64("dir_size", dirSize).
			Int64("max_bytes_total", config.MaxBytesTotal).
			Msg("Bytes in directory exceeds max")
		log.Debug().
			Str("biggest_file_id", biggestFileID).
			Msg("Removing file")
		os.RemoveAll(path.Join(config.ContentDirectory, biggestFileID))
	}
	log.Warn().Msg("TrimContent reached maximum iterations. Directory may still exceed limit.")
}

// DirSize returns the size of a directory in bytes and biggestFileID
func DirSize(path string) (int64, string, error) {
	var size int64
	biggestFileID := ""
	biggestFileSize := int64(0)
	err := filepath.Walk(path, func(pathName string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
			if info.Size() > biggestFileSize {
				pathName = filepath.ToSlash(pathName)
				biggestFileID = strings.Split(pathName, "/")[1]
				biggestFileSize = info.Size()
			}
		}
		return err
	})
	return size, biggestFileID, err
}

// copyToContentDirectory will move the temp file to the content directory and calculate
// the hash for generating the ID. It will also save the meta information in the content
// directory (the .json.gz files).
func copyToContentDirectory(fname string, tempFname string, originalSize uint64, config config.Config) (fnameFull string, err error) {
	defer os.Remove(tempFname)
	defer func() {
		go TrimContent(config)
	}()

	hash, _ := pkg.FileMD5(tempFname)

	id := pkg.RandomName(hash)

	destDir := path.Join(config.ContentDirectory, id)
	if _, err := os.Stat(destDir); !os.IsNotExist(err) {
		err = os.RemoveAll(destDir)
		if err != nil {
			log.Error().Err(err).Msg("Error removing existing directory")
			return "", err
		}
	}

	if err := os.MkdirAll(destDir, os.ModePerm); err != nil {
		log.Error().Err(err).Msg("Error creating directory")
		return "", err
	}

	err = os.Rename(tempFname, path.Join(config.ContentDirectory, id, fname))
	if err != nil {
		log.Error().Err(err).Msg("Error renaming file")
		return
	}

	log.Debug().Msgf("Moved to %s", fnameFull)

	fnameFull = path.Join(id, fname)

	page := NewPage(config)
	page.ID = id
	page.Hash = hash
	page.Name = fname
	page.Size = originalSize
	page.SizeHuman = humanize.Bytes(originalSize)
	page.Modified = time.Now()
	page.ModifiedHuman = humanize.Time(page.Modified)
	page.Link = fmt.Sprintf("/1/%s/%s", page.ID, page.Name)
	var isASCIIIData bool
	page.ContentType, isASCIIIData, err = pkg.GetFileContentType(path.Join(config.ContentDirectory, page.ID, page.Name))
	if err != nil {
		log.Error().Err(err).Msg("Error getting content type")
		return
	}
	page.IsImage = strings.Contains(page.ContentType, "image/")
	page.IsText = strings.Contains(page.ContentType, "text/")
	page.IsAudio = strings.Contains(page.ContentType, "audio/")
	page.IsVideo = strings.Contains(page.ContentType, "video/")
	page.IsASCII = isASCIIIData

	metaFilePath := path.Join(destDir, id+".json.gz")
	if err := writeGzippedJSON(page, metaFilePath); err != nil {
		log.Error().Err(err).Msg("Error writing JSON metadata")
		return "", err
	}

	return
}

// writeGzippedJSON writes the given data as gzipped JSON to the specified file path.
func writeGzippedJSON(data interface{}, filePath string) error {
	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	gzWriter := gzip.NewWriter(file)
	defer gzWriter.Close()

	encoder := json.NewEncoder(gzWriter)
	encoder.SetIndent("", " ") // Optional: for pretty-printing

	if err := encoder.Encode(data); err != nil {
		return err
	}

	return gzWriter.Flush()
}
