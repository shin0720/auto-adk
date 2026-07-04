package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"

	"github.com/shin0720/auto-adk/content"
)

type fileEntry struct {
	Path string `json:"path"`
	Mod  int64  `json:"mod"`
}

// uiProjectRoot holds the working directory at server startup.
var uiProjectRoot string

// currentWorkspace tracks the active workspace directory, updated when the user
// navigates via the browser. Protected by a mutex to avoid races with concurrent requests.
var (
	currentWorkspaceMu   sync.RWMutex
	currentWorkspaceDir  string
)

// newUICmd는 웹 UI 서버를 실행하는 ui 서브커맨드를 생성한다.
func newUICmd() *cobra.Command {
	var port int
	var noBrowser bool

	cmd := &cobra.Command{
		Use:   "ui",
		Short: "Autopus 시각적 대시보드 실행",
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := os.Getwd()
			if err != nil {
				return err
			}
			uiProjectRoot = root
			currentWorkspaceMu.Lock()
			currentWorkspaceDir = root
			currentWorkspaceMu.Unlock()

			addr := fmt.Sprintf("localhost:%d", port)
			fmt.Printf("🐙 Autopus Studio v5.0 시작 중... http://%s\n", addr)

			http.HandleFunc("/api/workspace/change", handleWorkspaceChange)
			http.HandleFunc("/api/workflow/state", handleWorkflowState)
			http.HandleFunc("/api/workflow/stream", handleWorkflowStream)
			http.HandleFunc("/api/workflow/event", handleWorkflowEvent)
			http.HandleFunc("/api/workflow/run", handleWorkflowRun)
			http.HandleFunc("/api/workflow/cancel", handleWorkflowCancel)
			http.HandleFunc("/api/workflow/running", handleWorkflowRunning)
			http.HandleFunc("/api/workspace/list", handleWorkspaceList)
			http.HandleFunc("/api/files/list", handleFileList)
			http.HandleFunc("/api/files/read", handleFileRead)
			http.HandleFunc("/api/files/write", handleFileWrite)
			http.HandleFunc("/api/files/upload", handleFileUpload)
			http.HandleFunc("/api/providers/status", handleProviderStatus)
			http.HandleFunc("/api/providers/connect", handleProviderConnect)
			http.HandleFunc("/api/shutdown", handleShutdown)

			// Serve split static assets (CSS/JS) from the embedded ui directory.
			// Registered before the root handler so /ui/* routes resolve first.
			if staticFS, err := fs.Sub(content.FS, "ui"); err == nil {
				http.Handle("/ui/", http.StripPrefix("/ui/", http.FileServer(http.FS(staticFS))))
			}
			http.HandleFunc("/", handleDashboard)

			// Skip auto-opening the browser in audit/CI runs. Either the
			// --no-browser flag or a non-empty AUTOPUS_NO_BROWSER env var
			// suppresses it so verification runs never leave stray Chrome
			// --app windows behind.
			if noBrowser || os.Getenv("AUTOPUS_NO_BROWSER") != "" {
				fmt.Printf("🔇 no-browser 모드: 브라우저를 열지 않습니다. http://%s 에서 확인하세요.\n", addr)
			} else {
				go openBrowser("http://" + addr)
			}
			return http.ListenAndServe(addr, nil)
		},
	}

	cmd.Flags().IntVarP(&port, "port", "p", 8080, "서버 포트 번호")
	cmd.Flags().BoolVar(&noBrowser, "no-browser", false, "브라우저 자동 열기 비활성화 (검증/CI용)")
	return cmd
}

func handleWorkspaceChange(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path string `json:"path"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	resolved, err := resolveWorkspacePath(req.Path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	currentWorkspaceMu.Lock()
	currentWorkspaceDir = resolved
	currentWorkspaceMu.Unlock()

	_ = json.NewEncoder(w).Encode(map[string]string{"status": "success", "currentDir": resolved})
}

// getWorkspaceDir returns the current active workspace directory.
func getWorkspaceDir() string {
	currentWorkspaceMu.RLock()
	defer currentWorkspaceMu.RUnlock()
	return currentWorkspaceDir
}

func handleWorkspaceList(w http.ResponseWriter, r *http.Request) {
	dir := getWorkspaceDir()
	payload, err := workspaceListPayload(dir)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	_ = json.NewEncoder(w).Encode(payload)
}

func handleFileList(w http.ResponseWriter, r *http.Request) {
	root := getWorkspaceDir()
	entries, err := os.ReadDir(root)
	var files []fileEntry
	if err == nil {
		for _, entry := range entries {
			if !entry.IsDir() {
				var mod int64
				if info, e2 := entry.Info(); e2 == nil {
					mod = info.ModTime().Unix()
				}
				files = append(files, fileEntry{Path: entry.Name(), Mod: mod})
			}
		}
	}
	if files == nil {
		files = []fileEntry{}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(files)
}

func handleFileRead(w http.ResponseWriter, r *http.Request) {
	root := getWorkspaceDir()
	path := r.URL.Query().Get("path")
	content, _ := readWorkspaceFile(root, path)
	_, _ = w.Write(content)
}

func handleFileWrite(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Filename string `json:"filename"`
		Content  string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if req.Filename == "" || strings.ContainsAny(req.Filename, "/\\") {
		http.Error(w, "invalid filename", http.StatusBadRequest)
		return
	}
	root := getWorkspaceDir()
	path := filepath.Join(root, req.Filename)
	if err := os.WriteFile(path, []byte(req.Content), 0o644); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "success", "path": path})
}

func handleDashboard(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(renderDashboard())
}

// renderDashboard assembles the dashboard shell with the body partials injected
// at the <!--AUTOPUS_BODY--> marker. The body is split across two files to keep
// every embedded source file under the 300-line limit.
func renderDashboard() []byte {
	shell, _ := content.FS.ReadFile("ui/dashboard.html")
	body1, _ := content.FS.ReadFile("ui/dashboard-body-1.html")
	body2, _ := content.FS.ReadFile("ui/dashboard-body-2.html")
	body := append(body1, body2...)
	return bytes.Replace(shell, []byte("<!--AUTOPUS_BODY-->"), body, 1)
}

func handleShutdown(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.WriteHeader(http.StatusNoContent)
	go func() {
		time.Sleep(150 * time.Millisecond)
		os.Exit(0)
	}()
}

func openBrowser(url string) {
	var err error
	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	case "windows":
		err = exec.Command("cmd", "/c", "start", "chrome", "--app="+url).Start()
		if err != nil {
			err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
		}
	case "darwin":
		err = exec.Command("open", url).Start()
	}
	if err != nil {
		fmt.Printf("브라우저 열기 실패: %v\n", err)
	}
}

