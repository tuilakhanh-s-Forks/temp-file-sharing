package handlers

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"

	"github.com/tuilakhanh/webshare/internal/config"
)

// Page defines content that is available to each page
type Page struct {
	// properties of the file
	ID            string
	Name          string
	PathToFile    string
	Hash          string
	Link          string
	Size          uint64
	SizeHuman     string
	ContentType   string
	Modified      time.Time
	ModifiedHuman string
	IsImage       bool
	IsText        bool
	IsAudio       bool
	IsVideo       bool
	IsASCII       bool

	// computed properties
	NameOnDisk          string
	Text                string
	TimeToDeletion      time.Duration
	TimeToDeletionHuman string

	// page specific info
	Error string

	// Config data
	Config config.Config
}

func NewPage(config config.Config) (p *Page) {
	p = new(Page)
	p.Config = config
	return
}

func (p *Page) handlePost(c *gin.Context) (err error) {
	fileHeader, err := c.FormFile("file")
	if err != nil {
		log.Error().Err(err).Msg("Error getting file from form")
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	if fileHeader.Size > p.Config.MaxBytesPerFile {
		c.JSON(http.StatusBadRequest, gin.H{"message": fmt.Sprintf("Upload exceeds max file size: %s.", p.Config.MaxBytesPerFileHuman)})
		return
	}

	file, err := fileHeader.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Unable to open uploaded file"})
		return
	}
	defer file.Close()

	tempFile, err := os.CreateTemp(p.Config.ContentDirectory, "upload_")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Unable to create temporary file"})
		return
	}
	defer os.Remove(tempFile.Name())

	gzWriter := gzip.NewWriter(tempFile)
	defer gzWriter.Close()

	_, err = io.Copy(gzWriter, file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Unable to compress file"})
		return
	}

	err = gzWriter.Close()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Unable to close gzip writer"})
		return
	}

	finalname, err := copyToContentDirectory(fileHeader.Filename, tempFile.Name(), uint64(fileHeader.Size), p.Config)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Error processing file"})
		return
	}

	if err := tempFile.Close(); err != nil {
		log.Warn().Err(err).Msg("Error closing temporary file")
	}

	c.JSON(http.StatusCreated, gin.H{"id": finalname})
	return
}

func (p *Page) handleGetData(w http.ResponseWriter, decompress bool) (err error) {
	f, err := os.Open(p.NameOnDisk)
	if err != nil {
		log.Error().Err(err).Msg("Error opening file")
		return
	}
	if decompress {
		gzf, _ := gzip.NewReader(f)
		defer gzf.Close()
		io.Copy(w, gzf)
	} else {
		w.Header().Set("Content-Encoding", "gzip")
		w.Header().Set("Content-Type", p.ContentType)
		io.Copy(w, f)
	}
	return
}

func (p *Page) handleShowDataInBrowser(w http.ResponseWriter, tmpl *template.Template) (err error) {
	log.Debug().Interface("page_data", p).Msg("Page data")
	if p.IsASCII && p.Size < 10000000 {
		log.Debug().Str("page_id", p.ID).Msg("Showing page")

		file, err := os.Open(p.NameOnDisk)
		if err != nil {
			log.Error().Err(err).Msg("Error opening file")
			return err
		}
		defer file.Close()

		gr, err := gzip.NewReader(file)
		if err != nil {
			log.Error().Err(err).Msg("Error creating gzip reader")
			return err
		}
		defer gr.Close()

		buf := new(bytes.Buffer)
		_, err = io.Copy(buf, gr)
		if err != nil {
			log.Error().Err(err).Msg("Error reading from gzip reader")
			return err
		}

		p.Text = buf.String()
	}
	tmpl.Execute(w, p)
	return
}

func (p *Page) handleGetHome(w http.ResponseWriter, tmpl *template.Template) (err error) {
	return tmpl.Execute(w, p)
}
