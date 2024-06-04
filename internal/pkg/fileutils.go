package pkg

import (
	"compress/gzip"
	"crypto/md5"
	"encoding/hex"
	"io"
	"net/http"
	"os"
	"strings"
	"unicode"

	"github.com/h2non/filetype"
)

func FileMD5(pathToFile string) (result string, err error) {
	file, err := os.Open(pathToFile)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	checksum := hex.EncodeToString(hash.Sum(nil))
	return checksum, nil
}

// GetFileContentType returns the MIME content-type of a file.
func GetFileContentType(filename string) (string, bool, error) {
	file, err := os.Open(filename)
	if err != nil {
		return "", false, err
	}
	defer file.Close()
	return GetFileContentTypeReader(filename, file)
}

// GetFileContentTypeReader determines the content type from an io.Reader.
func GetFileContentTypeReader(filename string, reader io.Reader) (contentType string, isaciii bool, err error) {
	// Open the file
	gzReader, err := gzip.NewReader(reader)
	if err != nil {
		return "", false, err
	}
	defer gzReader.Close()

	// Read the file header for content type detection
	header := make([]byte, 261)
	_, err = gzReader.Read(header)
	if err != nil {
		return "", false, err
	}

	// Detect content type using the 'filetype' library
	kind, err := filetype.Match(header)
	if err != nil {
		return
	}

	if err != nil {
		return
	}
	if kind == filetype.Unknown {
		contentType = strings.Split(http.DetectContentType(header), ";")[0]
		if contentType == "application/octet-stream" {
			if isASCII(string(header)) {
				contentType = "text/plain"
			}
		}
	} else {
		contentType = kind.MIME.Value
	}

	// if we have a text file, then use the filename to force what the
	// content type should be
	if contentType == "text/plain" {
		if strings.Contains(filename, ".js") {
			contentType = "application/javascript"
		} else if strings.Contains(filename, ".css") {
			contentType = "text/css"
		}
	}
	isaciii = isASCII(string(header))
	return contentType, isaciii, nil
}

// isASCII checks if a string is entirely composed of ASCII characters.
func isASCII(s string) bool {
	for _, r := range s {
		if r > unicode.MaxASCII {
			return false
		}
	}
	return true
}
