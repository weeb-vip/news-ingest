package app

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/weeb-vip/news-ingest/config"
	"github.com/weeb-vip/news-ingest/internal/dedupe"
	"github.com/weeb-vip/news-ingest/internal/kafkax"
	"github.com/weeb-vip/news-ingest/internal/model"
)

// ServeAPI runs the HTTP ingest: POST /v1/news → normalize + dedupe → Kafka.
func ServeAPI() error {
	cfg := config.Load()
	prod := kafkax.NewProducer(cfg.Kafka.BootstrapServers)
	defer prod.Close()

	h := &apiHandler{cfg: cfg, prod: prod}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/news", h.postNews)
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	addr := ":" + cfg.App.Port
	slog.Info("news-ingest api listening", "addr", addr,
		"news_topic", cfg.Kafka.NewsTopic, "fanart_topic", cfg.Kafka.FanartTopic)
	return http.ListenAndServe(addr, mux)
}

type apiHandler struct {
	cfg  config.Config
	prod *kafkax.Producer
}

func (h *apiHandler) postNews(w http.ResponseWriter, r *http.Request) {
	if h.cfg.Ingest.Token != "" {
		got := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if got != h.cfg.Ingest.Token {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
	}

	var req model.IngestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.AnimeID) == "" {
		http.Error(w, "anime_id is required", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	news, fanart := 0, 0

	for _, n := range req.News {
		if strings.TrimSpace(n.Date) == "" {
			continue // dated items only
		}
		val, _ := json.Marshal(model.Envelope[model.NewsMessage]{Data: buildNewsMessage(req, n)})
		if err := h.prod.Write(ctx, h.cfg.Kafka.NewsTopic, []byte(req.AnimeID), val); err != nil {
			slog.Error("produce news failed", "err", err)
			http.Error(w, "produce failed", http.StatusBadGateway)
			return
		}
		news++
	}

	for _, u := range req.Fanart {
		u = strings.TrimSpace(u)
		if u == "" {
			continue
		}
		fm := model.FanartMessage{ID: dedupe.FanartID(req.AnimeID, u), AnimeID: req.AnimeID, ImageURL: u}
		val, _ := json.Marshal(model.Envelope[model.FanartMessage]{Data: fm})
		if err := h.prod.Write(ctx, h.cfg.Kafka.FanartTopic, []byte(req.AnimeID), val); err != nil {
			slog.Error("produce fanart failed", "err", err)
			http.Error(w, "produce failed", http.StatusBadGateway)
			return
		}
		fanart++
	}

	slog.Info("ingested", "anime_id", req.AnimeID, "news", news, "fanart", fanart)
	w.Header().Set("content-type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "news": news, "fanart": fanart})
}

func strptr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func buildNewsMessage(req model.IngestRequest, n model.NewsIn) model.NewsMessage {
	normURL := ""
	var srcURL *string
	if n.URL != nil && *n.URL != "" {
		normURL = dedupe.NormalizeURL(*n.URL)
		srcURL = n.URL
	}
	iso := dedupe.NormalizeDate(n.Date)
	slug := dedupe.TitleSlug(n.Title)
	return model.NewsMessage{
		ID:            dedupe.NewsID(req.AnimeID, normURL, iso, slug),
		AnimeID:       req.AnimeID,
		MalID:         req.MalID,
		Title:         n.Title,
		Summary:       strptr(n.Summary),
		Category:      n.Category,
		SourceURL:     srcURL,
		SourceName:    strptr(n.Source),
		PublishedDate: strptr(iso),
		EpisodeNumber: n.Episode,
		TitleSlug:     slug,
		ResearchedAt:  strptr(req.ResearchedAt),
		Language:      normalizeLanguage(n.Language),
		References:    cleanReferences(n.References),
	}
}

// normalizeLanguage keeps only a plausible ISO 639-1 code, lowercased. Anything else
// (prose, an empty string, a locale tail like "ja-JP") is reduced or dropped rather
// than stored — a junk language code is worse than none, since the UI acts on it.
func normalizeLanguage(raw *string) *string {
	if raw == nil {
		return nil
	}
	v := strings.ToLower(strings.TrimSpace(*raw))
	if i := strings.IndexAny(v, "-_"); i > 0 {
		v = v[:i]
	}
	if len(v) != 2 {
		return nil
	}
	for _, c := range v {
		if c < 'a' || c > 'z' {
			return nil
		}
	}
	return &v
}

// cleanReferences drops entries with no usable URL and caps the list. The research
// tool already limits these, but this service accepts input from outside itself.
func cleanReferences(in []model.Reference) []model.Reference {
	out := make([]model.Reference, 0, len(in))
	seen := map[string]bool{}
	for _, r := range in {
		u := strings.TrimSpace(r.URL)
		if u == "" || seen[u] {
			continue
		}
		if !strings.HasPrefix(u, "http://") && !strings.HasPrefix(u, "https://") {
			continue
		}
		seen[u] = true
		kind := strings.TrimSpace(r.Kind)
		if kind == "" {
			kind = "link"
		}
		out = append(out, model.Reference{Kind: kind, Title: strings.TrimSpace(r.Title), URL: u})
		if len(out) >= 8 {
			break
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
