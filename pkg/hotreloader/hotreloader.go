package hotreloader

import (
	"fmt"
	"html/template"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/fsnotify/fsnotify"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"golang.org/x/net/websocket"
)

type Logger interface {
	Print(i ...interface{})
	Printf(format string, args ...interface{})
	Fatal(i ...interface{})
	Fatalf(format string, args ...interface{})
	Panic(i ...interface{})
	Panicf(format string, args ...interface{})
}

type UpdateMessage struct {
	Path       string `json:"path"`
	AutoReload bool   `json:"autoReload"`
}

type HotReloader struct {
	staticFilePath   string
	templatePathGlob string
	staticRoute      string
	hotReload        bool
	autoReload       bool

	templates *template.Template
	logger    Logger
	watcher   *fsnotify.Watcher
	updates   chan UpdateMessage
	consumers sync.Map
}

type HotReloaderOption func(*HotReloader) error
type CloseFunction func()

func WithStaticFilePath(path string) HotReloaderOption {
	return func(m *HotReloader) error {
		m.staticFilePath = path
		if !strings.HasPrefix(m.staticFilePath, "./") {
			m.staticFilePath = "./" + m.staticFilePath
		}
		if !strings.HasSuffix(m.staticFilePath, "/") {
			m.staticFilePath = m.staticFilePath + "/"
		}
		return nil
	}
}

func WithTemplatePathGlob(path string) HotReloaderOption {
	return func(m *HotReloader) error {
		m.templatePathGlob = path
		if !strings.HasPrefix(m.templatePathGlob, "./") {
			m.templatePathGlob = "./" + m.templatePathGlob
		}
		return nil
	}
}

func WithStaticRoute(route string) HotReloaderOption {
	return func(m *HotReloader) error {
		m.staticRoute = route
		return nil
	}
}

func WithHotReload(val bool) HotReloaderOption {
	return func(m *HotReloader) error {
		m.hotReload = val
		return nil
	}
}

func WithAutoReload(val bool) HotReloaderOption {
	return func(m *HotReloader) error {
		m.autoReload = val
		return nil
	}
}

func WithLogger(logger echo.Logger) HotReloaderOption {
	return func(m *HotReloader) error {
		m.logger = logger
		return nil
	}
}

func New(opts ...HotReloaderOption) (*HotReloader, error) {
	middleware := &HotReloader{
		staticFilePath: "./static",
		hotReload:      true,
		autoReload:     true,
		logger:         log.Default(),
		updates:        make(chan UpdateMessage),
		consumers:      sync.Map{},
	}

	for _, opt := range opts {
		err := opt(middleware)
		if err != nil {
			return nil, err
		}
	}

	err := middleware.injectTemplates()
	if err != nil {
		return nil, fmt.Errorf("injecting templates: %w", err)
	}

	err = middleware.Run()
	if err != nil {
		return nil, err
	}

	middleware.logger.Print("webpack middleware initialized")
	return middleware, nil
}

func (m *HotReloader) SetLogger(logger echo.Logger) {
	m.logger = logger
}

func (m *HotReloader) IsHotReloadEnabled() bool {
	return m.hotReload
}

func (m *HotReloader) AddConsumer() (chan UpdateMessage, CloseFunction) {
	key := uuid.New().String()
	consumer := make(chan UpdateMessage)
	m.consumers.Store(key, consumer)

	return consumer, func() {
		m.consumers.Delete(key)
		close(consumer)
	}
}

func (m *HotReloader) watch() error {
	var err error

	m.watcher, err = fsnotify.NewWatcher()
	if err != nil {
		m.logger.Fatalf("watcher: %+v", err)
	}

	go func() {
		for {
			select {
			case event, ok := <-m.watcher.Events:
				if !ok {
					return
				}
				if event.Has(fsnotify.Write) {
					filePath := "./" + event.Name
					m.logger.Printf("modified file: %s", filePath)

					if matched, err := filepath.Match(m.templatePathGlob, filePath); err == nil && matched {
						err = m.injectTemplates()
						if err != nil {
							m.logger.Printf("watcher: %+v", err)
						}
						m.dispatchReload(filePath)
						continue
					}

					if strings.HasPrefix(filePath, m.staticFilePath) {
						m.dispatchReload(filePath)
						continue
					}
				}
			case err, ok := <-m.watcher.Errors:
				if !ok {
					return
				}
				m.logger.Printf("watcher: %+v", err)
			}
		}
	}()

	go func() {
		for msg := range m.updates {
			m.consumers.Range(func(key, value interface{}) bool {
				consumer := value.(chan UpdateMessage)
				consumer <- msg
				return true
			})
		}
		m.consumers.Range(func(key, value interface{}) bool {
			consumer := value.(chan UpdateMessage)
			close(consumer)
			return true
		})
	}()

	err = m.watchTree(m.staticFilePath)
	if err != nil {
		return fmt.Errorf("adding watcher for static path: %w", err)
	}

	dir, _ := filepath.Split(m.templatePathGlob)
	err = m.watchTree(dir)
	if err != nil {
		return fmt.Errorf("adding watcher for template path: %w", err)
	}

	return nil
}

func (m *HotReloader) watchTree(srcDir string) error {
	return filepath.Walk(srcDir, func(srcPath string, f os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if f.IsDir() {
			return m.watcher.Add(srcPath)
		}
		return nil
	})
}

func (m *HotReloader) Run() error {
	m.logger.Print("Hot reload middleware starting...")
	m.logger.Printf("Static file path: %s", m.staticFilePath)
	m.logger.Printf("Template path glob: %s", m.templatePathGlob)

	err := m.watch()
	if err != nil {
		return fmt.Errorf("watching files: %w", err)
	}

	return nil
}

func (m *HotReloader) Close() {
	m.logger.Print("webpack middleware closing")

	if m.watcher != nil {
		err := m.watcher.Close()
		if err != nil {
			m.logger.Printf("closing watcher: %w", err)
		}
	}

	close(m.updates)
}

func (m *HotReloader) Render(w io.Writer, name string, data interface{}, c echo.Context) error {
	return m.templates.ExecuteTemplate(w, name, data)
}

func (m *HotReloader) injectTemplates() error {
	// templates can't be replaced once they've executed, so we need to re-parse all the templates when one changes
	m.templates = nil

	filenames, err := filepath.Glob(m.templatePathGlob)
	if err != nil {
		return err
	}

	for _, filename := range filenames {
		if err := m.injectTemplate(filename); err != nil {
			return err
		}
	}

	return err
}

func (m *HotReloader) injectTemplate(srcPath string) error {
	name := filepath.Base(srcPath)
	data, err := os.ReadFile(srcPath)
	if err != nil {
		return err
	}

	src := string(data)
	if pos := strings.Index(src, "</head>"); pos >= 0 {
		src = src[:pos] + `<script src="/hotreload-go/reload.js"></script>` + src[pos:]
	}

	var tpl *template.Template
	if m.templates == nil {
		m.templates = template.New(name)
		tpl = m.templates
	} else {
		tpl = m.templates.New(name)
	}

	_, err = tpl.Parse(src)
	if err != nil {
		return err
	}

	return nil
}

func (m *HotReloader) dispatchReload(path string) {
	m.updates <- UpdateMessage{
		Path:       path,
		AutoReload: m.autoReload,
	}
}

func (m *HotReloader) WebSocketHandler() websocket.Handler {
	return websocket.Handler(func(ws *websocket.Conn) {
		defer ws.Close()

		consumer, closeFn := m.AddConsumer()
		defer closeFn()

		for msg := range consumer {
			err := websocket.JSON.Send(ws, msg)
			if err != nil {
				break
			}
		}
	})
}
