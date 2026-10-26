package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"workbench/internal/skills"
)

func (app *App) writeSkill(writer http.ResponseWriter, request *http.Request) {
	limit := int64(32768)
	if request.URL.Path == "/api/skill/save" || request.URL.Path == "/api/skill/preview" {
		limit = skills.MaxContentBytes*6 + 4096
	}
	decoder := json.NewDecoder(http.MaxBytesReader(writer, request.Body, limit))
	decoder.DisallowUnknownFields()
	decode := func(target any) error {
		if err := decoder.Decode(target); err != nil {
			return fmt.Errorf("%w A bounded JSON object with the expected fields is required.", skills.ErrInvalid)
		}
		if err := decoder.Decode(&struct{}{}); err != io.EOF {
			return fmt.Errorf("%w Send one JSON object.", skills.ErrInvalid)
		}
		return nil
	}

	var result any
	var err error
	switch request.URL.Path {
	case "/api/skill/save":
		var save skills.SaveRequest
		if err = decode(&save); err == nil {
			result, err = app.SkillFiles.Save(save)
		}
	case "/api/skill/duplicate":
		var copyRequest skills.CopyRequest
		if err = decode(&copyRequest); err == nil {
			result, err = app.SkillFiles.Duplicate(copyRequest)
		}
	case "/api/skill/preview":
		var preview struct {
			Content *string `json:"content"`
		}
		if err = decode(&preview); err == nil {
			if preview.Content == nil {
				err = fmt.Errorf("%w Markdown content is required.", skills.ErrInvalid)
			} else {
				result, err = skills.Render(*preview.Content)
			}
		}
	default:
		failure(writer, http.StatusNotFound, errors.New("Not found."))
		return
	}

	if err != nil {
		skillFailure(writer, err)
		return
	}
	if request.URL.Path != "/api/skill/preview" {
		app.skillsMu.Lock()
		app.skillsAt = time.Time{}
		app.skillsMu.Unlock()
	}

	respond(writer, http.StatusOK, result)
}

func (app *App) exportSkill(writer http.ResponseWriter, request *http.Request) {
	archive, err := app.SkillFiles.Export(request.URL.Query().Get("id"), request.URL.Query().Get("revision"))
	if err != nil {
		skillFailure(writer, err)
		return
	}

	writer.Header().Set("Content-Type", "application/zip")
	writer.Header().Set("Content-Disposition", `attachment; filename="skill.zip"`)
	writer.WriteHeader(http.StatusOK)
	_, _ = writer.Write(archive)
}

func skillFailure(writer http.ResponseWriter, err error) {
	status := http.StatusBadRequest
	switch {
	case errors.Is(err, skills.ErrNotFound):
		status = http.StatusNotFound
	case errors.Is(err, skills.ErrReadOnly):
		status = http.StatusForbidden
	case errors.Is(err, skills.ErrConflict):
		status = http.StatusConflict
	}

	failure(writer, status, err)
}
