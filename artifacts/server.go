package artifacts

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

type Server struct {
	mu      sync.Mutex
	server  *http.Server
	port    string
	baseDir string
}

func NewServer() *Server {
	return &Server{}
}

const consoleInterceptorJS = `<script>(function(){var o={log:console.log.bind(console),error:console.error.bind(console),warn:console.warn.bind(console),info:console.info.bind(console)};function s(l,a){o[l].apply(console,a);try{window.parent.postMessage({type:'iframe-console',level:l,args:Array.from(a).map(function(x){if(typeof x==='object'){try{return JSON.stringify(x)}catch(e){return String(x)}}return String(x)}),timestamp:Date.now()},'*')}catch(e){}}console.log=function(){s('log',arguments)};console.error=function(){s('error',arguments)};console.warn=function(){s('warn',arguments)};console.info=function(){s('info',arguments)};window.onerror=function(m,u,l,c){s('error',[m+' at '+u+':'+l+':'+c])};window.addEventListener('unhandledrejection',function(e){s('error',['Unhandled Promise Rejection: '+(e.reason?e.reason.stack||String(e.reason):String(e))])})})();</script>`

func injectConsoleInterceptor(html string) string {
	script := consoleInterceptorJS
	if idx := strings.Index(html, "<head>"); idx != -1 {
		return html[:idx+6] + script + html[idx+6:]
	}
	if idx := strings.Index(html, "<HEAD>"); idx != -1 {
		return html[:idx+6] + script + html[idx+6:]
	}
	if idx := strings.Index(html, "<html"); idx != -1 {
		closeIdx := strings.Index(html[idx:], ">")
		if closeIdx != -1 {
			pos := idx + closeIdx + 1
			return html[:pos] + "<head>" + script + "</head>" + html[pos:]
		}
	}
	return script + html
}

func generateDirectoryListing(dir string) string {
	var files []string
	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		rel, rerr := filepath.Rel(dir, path)
		if rerr != nil {
			return nil
		}
		files = append(files, filepath.ToSlash(rel))
		return nil
	})
	sort.Strings(files)

	var b strings.Builder
	b.WriteString("<!DOCTYPE html><html><head><meta charset='utf-8'><title>Files</title>")
	b.WriteString(consoleInterceptorJS)
	b.WriteString("<style>body{font-family:system-ui;background:#111b2e;color:#e2e8f0;padding:2rem}a{color:#63b3ed;text-decoration:none;padding:0.3rem 0.5rem;display:block;border-bottom:1px solid #2d3748}a:hover{background:#1a2744;text-decoration:none}h1{font-size:1.2rem;margin:0 0 1rem 0;color:#63b3ed}</style>")
	b.WriteString("</head><body><h1>Artifact Files</h1>")
	for _, f := range files {
		b.WriteString(fmt.Sprintf("<a href='/%s'>%s</a>", f, f))
	}
	b.WriteString("</body></html>")
	return b.String()
}

func cachedFileServer(dir string) http.Handler {
	fileServer := http.FileServer(http.Dir(dir))
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")

		urlPath := r.URL.Path

		isHTML := strings.HasSuffix(urlPath, ".html") || urlPath == "/" || urlPath == "/index.html"
		if isHTML {
			fsPath := urlPath
			if fsPath == "/" {
				fsPath = "/index.html"
			}
			localPath := filepath.Join(dir, filepath.FromSlash(fsPath))

			cleanDir := filepath.Clean(dir)
			cleanLocal := filepath.Clean(localPath)
			if cleanLocal != cleanDir && !strings.HasPrefix(cleanLocal, cleanDir+string(os.PathSeparator)) {
				http.NotFound(w, r)
				return
			}

			content, err := os.ReadFile(localPath)
			if err == nil {
				html := injectConsoleInterceptor(string(content))
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				_, _ = w.Write([]byte(html))
				return
			}

			if urlPath == "/" || urlPath == "/index.html" {
				listing := generateDirectoryListing(dir)
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				_, _ = w.Write([]byte(listing))
				return
			}
		}

		fileServer.ServeHTTP(w, r)
	})
	return mux
}

func (s *Server) Start(ctx context.Context, dir string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.server != nil {
		return "http://127.0.0.1:" + s.port, nil
	}

	s.baseDir = dir

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}

	s.port = fmt.Sprintf("%d", listener.Addr().(*net.TCPAddr).Port)
	s.server = &http.Server{Handler: cachedFileServer(dir)}

	go func() {
		<-ctx.Done()
		s.server.Close()
	}()

	go s.server.Serve(listener)

	return "http://127.0.0.1:" + s.port, nil
}

func (s *Server) UpdateDir(dir string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.baseDir = dir
	if s.server != nil {
		s.server.Handler = cachedFileServer(dir)
	}
}
