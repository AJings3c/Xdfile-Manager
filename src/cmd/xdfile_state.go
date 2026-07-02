package cmd

import (
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	vt "github.com/charmbracelet/x/vt"

	filepreview "github.com/s0x401/xdfile-manager/src/pkg/file_preview"
)

type xdfileTerminal struct {
	Cwd            string
	Title          string
	RemoteProfile  string
	Busy           bool
	Input          textinput.Model
	Viewport       viewport.Model
	Lines          []string
	Events         chan tea.Msg
	Session        *xdfileTerminalPTYSession
	Emulator       *vt.SafeEmulator
	StreamEmulator *vt.SafeEmulator
	ManagedCancel  func()

	Suggestions         []string
	SuggestionCursor    int
	SuggestionInput     string
	SuggestionDismissed bool

	ScrollOffset    int
	ViewWidth       int
	ViewHeight      int
	ViewportContent string

	History               []string
	HistoryItems          map[string]xdfileTerminalHistoryItem
	HistoryLog            []xdfileTerminalHistoryLogEntry
	HistoryDeleted        map[string]struct{}
	HistoryIndex          int
	HistoryDraft          string
	HistorySearchActive   bool
	HistorySearchDraft    string
	HistorySearchQuery    string
	HistorySearchMatches  []string
	HistorySearchCursor   int
	CommandHadOutput      bool
	PendingPanel          int
	PendingCwd            string
	PendingPolls          int
	PendingHistoryCommand string
	PendingHistoryCwd     string

	StartupSubmitPending bool
	StreamCanRewrite     bool

	Exclusive xdfileExclusiveTerminal
}

type xdfileLayout struct {
	menuButtons             []xdfileButtonRect
	footerButtons           []xdfileButtonRect
	startHubNavRects        []xdfileButtonRect
	startHubItemRects       []xdfileButtonRect
	menuItemRects           []xdfileButtonRect
	menuRect                xdfileRect
	panelRects              [2]xdfileRect
	terminalRect            xdfileRect
	terminalInputRect       xdfileRect
	terminalSuggestionRects []xdfileButtonRect
	exclusiveRect           xdfileRect
}

type xdfileClickState struct {
	panel int
	row   int
	at    time.Time
}

type xdfileScreen int

const (
	xdfileScreenWorkbench xdfileScreen = iota
	xdfileScreenStartHub
)

type xdfileStartHubNav int

const (
	xdfileStartHubNavLocal xdfileStartHubNav = iota
	xdfileStartHubNavHosts
	xdfileStartHubNavRecent
	xdfileStartHubNavSettings
)

type xdfileStartHubState struct {
	Nav          xdfileStartHubNav
	Cursor       int
	Search       string
	SearchActive bool
	LastClickRow int
	LastClickAt  time.Time
}

type xdfilePanelMouseState struct {
	Active     bool
	Panel      int
	StartIndex int
	LastIndex  int
	Ctrl       bool
	Dragging   bool
	BaseMarked map[string]struct{}
}

type xdfileHoverState struct {
	MenuAction   xdfileAction
	MenuItem     int
	FooterAction xdfileAction
	Panel        int
	PanelIndex   int
}

type xdfilePanelSearchState struct {
	Active  bool
	Panel   int
	Pattern string
}

type xdfilePanelFilterState struct {
	Active       bool
	Panel        int
	Query        string
	Scroll       int
	CursorBefore int
	ScrollBefore int
}

type xdfilePanelFuzzyState struct {
	Active       bool
	Panel        int
	Query        string
	Matches      []xdfilePanelFuzzyMatch
	Cursor       int
	CursorBefore int
	ScrollBefore int
}

type xdfileTerminalFocusState struct {
	Focused     bool
	AutoFocused bool
}

type xdfileTerminalResultMsg struct {
	Command         string
	Output          string
	Err             error
	Dir             string
	Clear           bool
	SyncActivePanel bool
}

type xdfileTerminalConsoleDoneMsg struct {
	Command string
	Dir     string
	Err     error
}

type xdfileFooterCtrlHintExpiredMsg struct {
	At time.Time
}

type xdfileStatusSpinnerTickMsg struct {
	At time.Time
}

type xdfileAutoRefreshMsg struct{}
type xdfileReturnFromUserScreenMsg struct{}

type xdfilePreviewContent struct {
	Text        string
	Description string
	Visual      bool
}

type xdfileTerminalLineMsg struct {
	Line     string
	Rewrite  bool
	Finalize bool
}

type xdfileTerminalCommandDoneMsg struct {
	Command  string
	Cwd      string
	Err      error
	Canceled bool
}

type xdfileAICommandDoneMsg struct {
	Prompt  string
	Command string
	Err     error
}

type xdfilePluginActionDoneMsg struct {
	Plugin   xdfilePluginManifest
	Response xdfilePluginResponse
	Err      error
}

type xdfileTerminalCommandPollMsg struct{}

type xdfileTerminalExitMsg struct {
	Err error
}

type xdfileTerminalCommandStartMsg struct {
	Command  string
	Dir      string
	Events   chan tea.Msg
	Cancel   func()
	Emulator *vt.SafeEmulator
}

type xdfileTerminalStreamScreenMsg struct{}

type xdfileTerminalStartResultMsg struct {
	Session       *xdfileTerminalPTYSession
	Err           error
	Dir           string
	Title         string
	RemoteProfile string
}

type xdfileExclusiveTerminal struct {
	Command string
	Cwd     string
	Title   string
	Events  chan tea.Msg
	Session *xdfileTerminalPTYSession
}

type xdfileExclusiveTerminalStartMsg struct {
	Command string
	Dir     string
	Session *xdfileTerminalPTYSession
	Err     error
}

type xdfileExclusiveTerminalScreenMsg struct{}

type xdfileExclusiveTerminalTitleMsg struct {
	Title string
}

type xdfileExclusiveTerminalCwdMsg struct {
	Cwd string
}

type xdfileExclusiveTerminalExitMsg struct {
	Err error
}

type xdfileClipboardWriteResultMsg struct {
	Err error
}

type xdfileClipboardTextWriteResultMsg struct {
	Paths []string
	Err   error
}

type xdfileRemoteClipboardCopyResultMsg struct {
	Paths        []string
	CacheDir     string
	Names        []string
	Err          error
	ClipboardErr error
}

type xdfileRemoteClipboardPasteDoneMsg struct {
	Pending    *xdfilePendingClipboardPaste
	TargetPath string
	TopLevel   bool
	Err        error
}

type xdfileRemotePanelCopyDownloadDoneMsg struct {
	Paths          []string
	CacheDir       string
	DestinationDir string
	Err            error
}

type xdfileLocalClipboardPasteDoneMsg struct {
	Pending      *xdfilePendingClipboardPaste
	SourcePath   string
	TargetPath   string
	TopLevel     bool
	Action       xdfileAction
	ReplacedPath string
	Err          error
}

type xdfilePanelDirState struct {
	Path    string
	ModTime time.Time
	Exists  bool
}

type xdfileDeleteUndoItem struct {
	OriginalPath string
	StagedPath   string
}

type xdfileDeleteUndoBatch struct {
	Root  string
	Items []xdfileDeleteUndoItem
	At    time.Time
}

type xdfileClipboardMoveUndoItem struct {
	OriginalPath string
	MovedPath    string
	ReplacedPath string
}

type xdfileClipboardMoveUndoBatch struct {
	Root  string
	Items []xdfileClipboardMoveUndoItem
	At    time.Time
}

type xdfilePendingClipboardPaste struct {
	Sources              []string
	VirtualSources       []xdfileShellClipboardFile
	CutMode              bool
	DestinationDir       string
	CacheDir             string
	Queue                []xdfilePendingClipboardPasteItem
	ConflictSource       string
	ConflictTarget       string
	ConflictTopLevel     bool
	ConflictVirtualIndex int
	ConflictApplyAll     bool
	ConflictPolicy       xdfileAction
	Targets              []string
	RemainingSources     []string
	Skipped              int
	Overwritten          int
	Renamed              int
	LastTarget           string
	FocusTarget          string
	MoveUndoRoot         string
	MoveUndoItems        []xdfileClipboardMoveUndoItem
}

type xdfilePendingClipboardPasteItem struct {
	SourcePath   string
	TargetPath   string
	TopLevel     bool
	CleanupDir   bool
	Virtual      bool
	VirtualIndex int
}

type xdfilePendingArchive struct {
	SourcePaths []string
	TargetPath  string
	PanelIndex  int
}

type xdfilePendingExtract struct {
	SourcePath string
	TargetPath string
	PanelIndex int
}

type xdfileBatchRenameItem struct {
	SourcePath string
	TargetPath string
	OldName    string
	NewName    string
}

type xdfilePendingBatchRename struct {
	Items      []xdfileBatchRenameItem
	PanelIndex int
	Template   string
}

type xdfileZoxideCandidate struct {
	Path string
}

type xdfileZoxideQueryDoneMsg struct {
	Query      string
	PanelIndex int
	Candidates []xdfileZoxideCandidate
	Err        error
}

type xdfileMD5ChecksumDoneMsg struct {
	Path     string
	Name     string
	Checksum string
	Size     int64
	Err      error
}

type xdfileModel struct {
	width  int
	height int

	screen              xdfileScreen
	startHub            xdfileStartHubState
	activePanel         int
	terminalFocused     bool
	terminalAutoFocused bool
	terminalExpanded    bool
	terminalReturnFocus xdfileTerminalFocusState
	userScreenVisible   bool
	showHidden          bool

	panels             [2]xdfilePanel
	panelDirState      [2]xdfilePanelDirState
	terminal           xdfileTerminal
	modal              xdfileModal
	imagePreviewer     *filepreview.ImagePreviewer
	thumbnailGenerator *filepreview.ThumbnailGenerator
	layout             xdfileLayout
	layoutPrefs        xdfileLayoutPrefs
	themeCatalog       []xdfileTheme
	keymap             xdfileKeymap
	zoxideEnabled      bool
	aiConfig           xdfileAIConfig
	layoutFile         string
	commandsFile       string
	netboxFile         string
	pinsFile           string
	pluginsDir         string
	netboxConnections  []xdfileNetBoxConnection
	pins               []xdfilePin
	plugins            []xdfilePluginManifest
	quickView          xdfileQuickView
	workspaces         []xdfileWorkspace
	activeWorkspace    int

	statusText             string
	statusError            bool
	statusSpinnerIndex     int
	backgroundTaskBusy     bool
	lastClick              xdfileClickState
	panelMouse             xdfilePanelMouseState
	clipboardPath          string
	clipboardPaths         []string
	clipboardTextPaths     []string
	clipboardCut           bool
	openMenu               xdfileAction
	menuCursor             int
	contextMenu            xdfileMenu
	contextMenuAnchor      xdfileRect
	footerCtrlHintUntil    time.Time
	commandMenuPath        []int
	commandInsertPath      []int
	commandInsertIndex     int
	commandEditPath        []int
	commandEditIndex       int
	commandPromptHistory   map[string]string
	pendingCommandMenu     *xdfilePendingCommandMenu
	commandMenuTempFiles   []string
	remoteClipboardDirs    []string
	deleteUndoStack        []xdfileDeleteUndoBatch
	clipboardMoveUndoStack []xdfileClipboardMoveUndoBatch
	pendingClipboardPaste  *xdfilePendingClipboardPaste
	pendingArchive         *xdfilePendingArchive
	pendingExtract         *xdfilePendingExtract
	pendingBatchRename     *xdfilePendingBatchRename
	pendingAICommand       string
	pendingPluginAction    *xdfilePluginActionDoneMsg
	zoxideCandidates       []xdfileZoxideCandidate
	fileOperationCancel    func()
	fileOperationProgress  *xdfileFileOperationProgress
	fileOperationQueue     []xdfileFileOperation
	terminalStarting       bool
	hover                  xdfileHoverState
	panelSearch            xdfilePanelSearchState
	panelFilter            xdfilePanelFilterState
	panelFuzzy             xdfilePanelFuzzyState
}

var xdfileRenderPreviewThumbnailFunc = func(m *xdfileModel, path string, width int, height int) (string, bool, error) {
	return m.renderPreviewThumbnail(path, width, height)
}
