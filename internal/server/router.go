package server

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/hwhang0917/local-eml/internal/importer"
	"github.com/hwhang0917/local-eml/internal/store"
)

type Server struct {
	Store    *store.Store
	Importer *importer.Importer
	Hub      *importer.Hub
}

func (s *Server) Router() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)

	r.Get("/healthz", s.handleHealth)

	r.Route("/api", func(api chi.Router) {
		api.Post("/imports", s.handleImportUpload)
		api.Post("/imports/s3", s.handleImportS3)
		api.Post("/imports/imap", s.handleImportImap)
		api.Get("/imports/{id}", s.handleImportStatus)
		api.Get("/imports/{id}/events", s.handleImportEvents)
		api.Get("/emails", s.handleListEmails)
		api.Get("/emails/{sha}", s.handleGetEmail)
		api.Get("/emails/{sha}/raw", s.handleEmailRaw)
		api.Get("/emails/{sha}/parts", s.handleEmailParts)
		api.Get("/emails/{sha}/text", s.handleEmailText)
		api.Get("/emails/{sha}/html", s.handleEmailHTML)
		api.Get("/emails/{sha}/cid/{cid}", s.handleEmailCID)
		api.Get("/emails/{sha}/attachments/{idx}", s.handleEmailAttachment)
		api.Put("/emails/{sha}/star", s.handleStarEmail)
		api.Delete("/emails/{sha}/star", s.handleUnstarEmail)

		api.Get("/imap/profiles", s.handleListIMAPProfiles)
		api.Post("/imap/profiles", s.handleSaveIMAPProfile)
		api.Delete("/imap/profiles/{id}", s.handleDeleteIMAPProfile)
	})

	r.Handle("/*", spaHandler())

	return r
}
