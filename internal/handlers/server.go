package handlers

import (
	"bytes"
	"compress/gzip"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/gin-contrib/logger"
	"github.com/gin-gonic/gin"
	"github.com/hako/durafmt"
	"github.com/rs/zerolog/log"

	"github.com/tuilakhanh/webshare/internal/config"
	"github.com/tuilakhanh/webshare/internal/pkg"
)

//go:embed static/*
var content embed.FS

type Server struct {
	config        *config.Config
	indexTemplate *template.Template
}

func NewServer(cfg *config.Config) *Server {
	tmpl, err := template.ParseFS(content, "static/index.html")
	if err != nil {
		log.Fatal().Err(err).Msg("Error parsing index template")
	}
	return &Server{
		config:        cfg,
		indexTemplate: tmpl,
	}
}

func (s *Server) Start() error {
	go func() {
		s.deleteOld(true) // Initial cleanup on startup
		pkg.TrimContent(*s.config)
		for {
			time.Sleep(30 * time.Minute)
			s.deleteOld()
			pkg.TrimContent(*s.config)
		}
	}()
	router := gin.Default()
	router.Use(logger.SetLogger())
	s.SetupRoutes(router)
	return router.Run(":" + s.config.Port)
}

func (s *Server) SetupRoutes(router *gin.Engine) { // Method on your server struct
	router.GET("/", s.handleHome)
	router.GET("/delete/:id", s.handleDelete)
	router.GET("/exists/:id/:name", s.handleExists)
	router.GET("/static/*filepath", s.handleStatic)
	router.GET("/1/:id/:name", s.handleRawData) // Assuming raw data doesn't need decompression
	router.GET("/:id/:name", s.handleShowData)  // Showing data in the browser
	router.POST("/", s.handlePost)
}

func (s *Server) handleHome(c *gin.Context) {
	p := NewPage(*s.config)
	p.handleGetHome(c.Writer, s.indexTemplate)
}

func (s *Server) handleDelete(c *gin.Context) {
	// GET /delete/ID will delete the ID
	id := c.Param("id")
	_, errStat := os.Stat(path.Join(s.config.ContentDirectory, id))
	if errStat != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("Data with id '%s' does not exist.", id)})
		return
	}
	os.RemoveAll(path.Join(s.config.ContentDirectory, id))
	p := NewPage(*s.config)
	p.Error = fmt.Sprintf("Removed %s.", id)
	p.handleGetHome(c.Writer, s.indexTemplate)
}

func (s *Server) handleExists(c *gin.Context) {
	id := filepath.Clean(c.Param("id"))
	name := filepath.Clean(c.Param("name"))

	// Construct the full file path
	filePath := path.Join(s.config.ContentDirectory, id, name)

	// Check for existence directly using os.Stat
	_, err := os.Stat(filePath)

	response := gin.H{
		"exists": "no",
		"id":     id,
		"name":   name,
	}

	if !os.IsNotExist(err) {
		response["exists"] = "yes"
	}

	// Log the response
	log.Debug().Str("id", id).Str("name", name).Str("exists", response["exists"].(string)).Msg("Checking file existence")

	c.JSON(http.StatusOK, response)
}

func (s *Server) handleStatic(c *gin.Context) {
	page := NewPage(*s.config)
	page.NameOnDisk = strings.TrimPrefix(filepath.ToSlash(filepath.Clean(c.Request.URL.Path[1:])), "/") + ".gz"
	var b []byte
	b, err := content.ReadFile(page.NameOnDisk)
	if err != nil {
		log.Error().Err(err).Msg("Error reading file")
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	page.ContentType, _, err = pkg.GetFileContentTypeReader(page.NameOnDisk, bytes.NewBuffer(b))
	if err != nil {
		log.Error().Err(err).Msg("Error getting content type")
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	log.Debug().
		Str("file", page.NameOnDisk).
		Str("content_type", page.ContentType).
		Msg("Serving static file")
	c.Header("Content-Encoding", "gzip")
	c.Header("Content-Type", page.ContentType)
	_, err = c.Writer.Write(b)
	if err != nil {
		log.Warn().Err(err).Msg("Error writing response")
	}
}

func (s *Server) handleRawData(c *gin.Context) {
	id := c.Param("id")
	name := c.Param("name")

	// Construct the file path
	filePath := path.Join(s.config.ContentDirectory, id)

	// Check if file exists
	if _, err := os.Stat(filePath); err != nil {
		if os.IsNotExist(err) { // Handle specific case of missing file
			c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("Data with id '%s' does not exist.", id)})
		} else { // Handle other errors
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to access file"})
		}
		return
	}

	// Load page info and handle data
	page, err := loadPageInfo(id, *s.config)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Data with id '%s' does not exist.", id)})
		return
	}

	// Ensure the requested file matches the loaded page info
	if name == "" || name != page.Name {
		c.Redirect(http.StatusNotFound, fmt.Sprintf("/%s/%s", page.ID, page.Name))
		return
	}

	page.handleGetData(c.Writer, false)
}

func (s *Server) handleShowData(c *gin.Context) {
	id := c.Param("id")
	name := c.Param("name")

	// Load page info and handle potential errors
	page, err := loadPageInfo(id, *s.config)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Data with id '%s' does not exist.", id)})
		return
	}

	// Ensure the requested file matches the loaded page info
	if name != page.Name {
		c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("Data with id '%s' does not exist.", id)})
		return
	}

	page.handleShowDataInBrowser(c.Writer, s.indexTemplate) // Show data in browser
}

func (s *Server) handlePost(c *gin.Context) {
	page := NewPage(*s.config)
	err := page.handlePost(c)

	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	}
}

func loadPageInfo(id string, config config.Config) (p *Page, err error) {
	p = NewPage(config)
	f, err := os.Open(path.Join(config.ContentDirectory, id, id+".json.gz"))
	if err != nil {
		return
	}
	defer f.Close()

	w, err := gzip.NewReader(f)
	if err != nil {
		return nil, err
	}
	defer w.Close()

	err = json.NewDecoder(w).Decode(&p)
	if err != nil {
		return nil, err
	}

	p.NameOnDisk = path.Join(config.ContentDirectory, p.ID, p.Name)
	p.TimeToDeletion = (time.Duration(config.MinutesPerGigabyte) * time.Minute) * time.Duration(1000000000/p.Size)
	p.TimeToDeletionHuman = durafmt.Parse(p.TimeToDeletion).String()
	p.ModifiedHuman = humanize.Time(p.Modified)
	return
}

// deleteOld deletes old files from the content directory.
func (s *Server) deleteOld(removeTempFiles ...bool) {
	dirSize, _, err := pkg.DirSize(s.config.ContentDirectory)
	if err != nil {
		log.Error().Err(err).Msg("Error getting directory size")
		return
	}

	files, err := os.ReadDir(s.config.ContentDirectory)
	if err != nil {
		log.Error().Err(err).Msg("Error reading directory")
		return
	}

	log.Debug().Int("num_files", len(files)).
		Str("total_size", humanize.Bytes(uint64(dirSize))).
		Msg("Checking for old files")

	for _, f := range files {
		if strings.HasPrefix(f.Name(), "upload_") {
			if len(removeTempFiles) > 0 && removeTempFiles[0] {
				err := os.Remove(path.Join(s.config.ContentDirectory, f.Name()))
				if err != nil {
					log.Error().Err(err).Str("filename", f.Name()).Msg("Error removing temp file")
				}
			}
			continue
		}

		_, id := filepath.Split(f.Name())
		p, err := loadPageInfo(id, *s.config) // Pass config to loadPageInfo
		if err != nil {
			log.Debug().Err(err).Str("id", id).Msg("Skipping file: error loading page info")
			continue
		}

		age := time.Since(p.Modified)
		if age < p.TimeToDeletion {
			log.Debug().
				Str("id", id).
				Dur("age", age).
				Dur("time_to_deletion", p.TimeToDeletion).
				Msg("Skipping file: not old enough")
			continue
		}

		log.Info().
			Str("id", p.ID).
			Str("size", p.SizeHuman).
			Time("modified", p.Modified).
			Msg("Deleting old file")

		err = os.RemoveAll(path.Join(s.config.ContentDirectory, p.ID))
		if err != nil {
			log.Error().Err(err).Str("id", p.ID).Msg("Error deleting file")
		}
	}
}
