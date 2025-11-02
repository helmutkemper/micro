package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"runtime"
	"runtime/pprof"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/go-errors/errors"
	"github.com/helmutkemper/micro/v2/internal/action"
	"github.com/helmutkemper/micro/v2/internal/buffer"
	"github.com/helmutkemper/micro/v2/internal/clipboard"
	"github.com/helmutkemper/micro/v2/internal/config"
	"github.com/helmutkemper/micro/v2/internal/screen"
	"github.com/helmutkemper/micro/v2/internal/shell"
	"github.com/helmutkemper/micro/v2/internal/util"
	isatty "github.com/mattn/go-isatty"
	"github.com/micro-editor/tcell/v2"
	lua "github.com/yuin/gopher-lua"
)

// usado para detectar a sequência Esc,m (macOS) como Alt-M
var helLastEsc time.Time
var helLastEscForm time.Time // para detectar Esc,f como Alt-F

// flags
var (
	flagVersion   = flag.Bool("version", false, "Show the version number and information")
	flagConfigDir = flag.String("config-dir", "", "Specify a custom location for the configuration directory")
	flagOptions   = flag.Bool("options", false, "Show all option help")
	flagDebug     = flag.Bool("debug", false, "Enable debug mode (prints debug info to ./log.txt)")
	flagProfile   = flag.Bool("profile", false, "Enable CPU profiling (writes profile info to ./micro.prof)")
	flagPlugin    = flag.String("plugin", "", "Plugin command")
	flagClean     = flag.Bool("clean", false, "Clean configuration directory")
	optionFlags   map[string]*string

	sighup    chan os.Signal
	timerChan chan func()
)

func InitFlags() {
	flag.Usage = func() {
		fmt.Println("Usage: micro [OPTION]... [FILE]... [+LINE[:COL]] [+/REGEX]")
		fmt.Println("       micro [OPTION]... [FILE[:LINE[:COL]]]...  (only if the `parsecursor` option is enabled)")
		fmt.Println("-clean")
		fmt.Println("    \tClean the configuration directory and exit")
		fmt.Println("-config-dir dir")
		fmt.Println("    \tSpecify a custom location for the configuration directory")
		fmt.Println("FILE:LINE[:COL] (only if the `parsecursor` option is enabled)")
		fmt.Println("FILE +LINE[:COL]")
		fmt.Println("    \tSpecify a line and column to start the cursor at when opening a buffer")
		fmt.Println("+/REGEX")
		fmt.Println("    \tSpecify a regex to search for when opening a buffer")
		fmt.Println("-options")
		fmt.Println("    \tShow all options help and exit")
		fmt.Println("-debug")
		fmt.Println("    \tEnable debug mode (enables logging to ./log.txt)")
		fmt.Println("-profile")
		fmt.Println("    \tEnable CPU profiling (writes profile info to ./micro.prof")
		fmt.Println("    \tso it can be analyzed later with \"go tool pprof micro.prof\")")
		fmt.Println("-version")
		fmt.Println("    \tShow the version number and information and exit")

		fmt.Print("\nMicro's plugins can be managed at the command line with the following commands.\n")
		fmt.Println("-plugin install [PLUGIN]...")
		fmt.Println("    \tInstall plugin(s)")
		fmt.Println("-plugin remove [PLUGIN]...")
		fmt.Println("    \tRemove plugin(s)")
		fmt.Println("-plugin update [PLUGIN]...")
		fmt.Println("    \tUpdate plugin(s) (if no argument is given, updates all plugins)")
		fmt.Println("-plugin search [PLUGIN]...")
		fmt.Println("    \tSearch for a plugin")
		fmt.Println("-plugin list")
		fmt.Println("    \tList installed plugins")
		fmt.Println("-plugin available")
		fmt.Println("    \tList available plugins")

		fmt.Print("\nMicro's options can also be set via command line arguments for quick\nadjustments. For real configuration, please use the settings.json\nfile (see 'help options').\n\n")
		fmt.Println("-<option> value")
		fmt.Println("    \tSet `option` to `value` for this session")
		fmt.Println("    \tFor example: `micro -syntax off file.c`")
		fmt.Println("\nUse `micro -options` to see the full list of configuration options")
	}

	optionFlags = make(map[string]*string)
	for k, v := range config.DefaultAllSettings() {
		optionFlags[k] = flag.String(k, "", fmt.Sprintf("The %s option. Default value: '%v'.", k, v))
	}

	flag.Parse()

	if *flagVersion {
		fmt.Println("Version:", util.Version)
		fmt.Println("Commit hash:", util.CommitHash)
		fmt.Println("Compiled on", util.CompileDate)
		exit(0)
	}
	if *flagOptions {
		var keys []string
		m := config.DefaultAllSettings()
		for k := range m {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			v := m[k]
			fmt.Printf("-%s value\n", k)
			fmt.Printf("    \tDefault value: '%v'\n", v)
		}
		exit(0)
	}
	if util.Debug == "OFF" && *flagDebug {
		util.Debug = "ON"
	}
}

func DoPluginFlags() {
	if *flagClean || *flagPlugin != "" {
		config.LoadAllPlugins()
		if *flagPlugin != "" {
			args := flag.Args()
			config.PluginCommand(os.Stdout, *flagPlugin, args)
		} else if *flagClean {
			CleanConfig()
		}
		exit(0)
	}
}

func LoadInput(args []string) []*buffer.Buffer {
	var filename string
	var input []byte
	var err error
	buffers := make([]*buffer.Buffer, 0, len(args))

	btype := buffer.BTDefault
	if !isatty.IsTerminal(os.Stdout.Fd()) {
		btype = buffer.BTStdout
	}

	files := make([]string, 0, len(args))

	flagStartPos := buffer.Loc{-1, -1}
	posFlagr := regexp.MustCompile(`^\+(\d+)(?::(\d+))?$`)
	posIndex := -1

	searchText := ""
	searchFlagr := regexp.MustCompile(`^\+\/(.+)$`)
	searchIndex := -1

	for i, a := range args {
		posMatch := posFlagr.FindStringSubmatch(a)
		if len(posMatch) == 3 && posMatch[2] != "" {
			line, err := strconv.Atoi(posMatch[1])
			if err != nil {
				screen.TermMessage(err)
				continue
			}
			col, err := strconv.Atoi(posMatch[2])
			if err != nil {
				screen.TermMessage(err)
				continue
			}
			flagStartPos = buffer.Loc{col - 1, line - 1}
			posIndex = i
		} else if len(posMatch) == 3 && posMatch[2] == "" {
			line, err := strconv.Atoi(posMatch[1])
			if err != nil {
				screen.TermMessage(err)
				continue
			}
			flagStartPos = buffer.Loc{0, line - 1}
			posIndex = i
		} else {
			searchMatch := searchFlagr.FindStringSubmatch(a)
			if len(searchMatch) == 2 {
				searchText = searchMatch[1]
				searchIndex = i
			} else {
				files = append(files, a)
			}
		}
	}

	command := buffer.Command{
		StartCursor:      flagStartPos,
		SearchRegex:      searchText,
		SearchAfterStart: searchIndex > posIndex,
	}

	if len(files) > 0 {
		for i := 0; i < len(files); i++ {
			buf, err := buffer.NewBufferFromFileWithCommand(files[i], btype, command)
			if err != nil {
				screen.TermMessage(err)
				continue
			}
			buffers = append(buffers, buf)
		}
	} else if !isatty.IsTerminal(os.Stdin.Fd()) {
		input, err = io.ReadAll(os.Stdin)
		if err != nil {
			screen.TermMessage("Error reading from stdin: ", err)
			input = []byte{}
		}
		buffers = append(buffers, buffer.NewBufferFromStringWithCommand(string(input), filename, btype, command))
	} else {
		buffers = append(buffers, buffer.NewBufferFromStringWithCommand(string(input), filename, btype, command))
	}

	return buffers
}

func checkBackup(name string) error {
	target := filepath.Join(config.ConfigDir, name)
	backup := target + util.BackupSuffix
	if info, err := os.Stat(backup); err == nil {
		input, err := os.ReadFile(backup)
		if err == nil {
			t := info.ModTime()
			msg := fmt.Sprintf(buffer.BackupMsg, target, t.Format("Mon Jan _2 at 15:04, 2006"), backup)
			choice := screen.TermPrompt(msg, []string{"r", "i", "a", "recover", "ignore", "abort"}, true)

			if choice%3 == 0 {
				if err := os.WriteFile(target, input, util.FileMode); err != nil {
					return err
				}
				return os.Remove(backup)
			} else if choice%3 == 1 {
				return os.Remove(backup)
			} else if choice%3 == 2 {
				return errors.New("Aborted")
			}
		}
	}
	return nil
}

func exit(rc int) {
	for _, b := range buffer.OpenBuffers {
		if !b.Modified() {
			b.Fini()
		}
	}
	if screen.Screen != nil {
		screen.Screen.Fini()
	}
	os.Exit(rc)
}

func main() {
	defer func() {
		if util.Stdout.Len() > 0 {
			fmt.Fprint(os.Stdout, util.Stdout.String())
		}
		exit(0)
	}()

	InitFlags()

	if *flagProfile {
		f, err := os.Create("micro.prof")
		if err != nil {
			log.Fatal("error creating CPU profile: ", err)
		}
		if err := pprof.StartCPUProfile(f); err != nil {
			log.Fatal("error starting CPU profile: ", err)
		}
		defer pprof.StopCPUProfile()
	}

	InitLog()

	if err := config.InitConfigDir(*flagConfigDir); err != nil {
		screen.TermMessage(err)
	}

	config.InitRuntimeFiles(true)
	config.InitPlugins()

	if err := checkBackup("settings.json"); err != nil {
		screen.TermMessage(err)
		exit(1)
	}

	if err := config.ReadSettings(); err != nil {
		screen.TermMessage(err)
	}
	if err := config.InitGlobalSettings(); err != nil {
		screen.TermMessage(err)
	}

	// flags de opções de sessão
	for k, v := range optionFlags {
		if *v != "" {
			nativeValue, err := config.GetNativeValue(k, *v)
			if err != nil {
				screen.TermMessage(err)
				continue
			}
			if err = config.OptionIsValid(k, nativeValue); err != nil {
				screen.TermMessage(err)
				continue
			}
			config.GlobalSettings[k] = nativeValue
			config.VolatileSettings[k] = true
		}
	}

	DoPluginFlags()

	if err := screen.Init(); err != nil {
		fmt.Println(err)
		fmt.Println("Fatal: Micro could not initialize a Screen.")
		exit(1)
	}
	m := clipboard.SetMethod(config.GetGlobalOption("clipboard").(string))
	clipErr := clipboard.Initialize(m)

	defer func() {
		if err := recover(); err != nil {
			if screen.Screen != nil {
				screen.Screen.Fini()
			}
			if e, ok := err.(*lua.ApiError); ok {
				fmt.Println("Lua API error:", e)
			} else {
				fmt.Println("Micro encountered an error:", errors.Wrap(err, 2).ErrorStack(), "\nIf you can reproduce this error, please report it at https://github.com/helmutkemper/micro/issues")
			}
			for _, b := range buffer.OpenBuffers {
				if b.Modified() {
					b.Backup()
				}
			}
			exit(1)
		}
	}()

	if err := config.LoadAllPlugins(); err != nil {
		screen.TermMessage(err)
	}

	if err := checkBackup("bindings.json"); err != nil {
		screen.TermMessage(err)
		exit(1)
	}

	action.InitBindings()
	action.InitCommands()

	if err := config.RunPluginFn("preinit"); err != nil {
		screen.TermMessage(err)
	}

	action.InitGlobals()
	buffer.SetMessager(action.InfoBar)
	args := flag.Args()
	b := LoadInput(args)
	if len(b) == 0 {
		screen.Screen.Fini()
		runtime.Goexit()
	}
	action.InitTabs(b)

	if err := config.RunPluginFn("init"); err != nil {
		screen.TermMessage(err)
	}
	if err := config.RunPluginFn("postinit"); err != nil {
		screen.TermMessage(err)
	}
	if err := config.InitColorscheme(); err != nil {
		screen.TermMessage(err)
	}
	if clipErr != nil {
		log.Println(clipErr, " or change 'clipboard' option")
	}

	config.StartAutoSave()
	if a := config.GetGlobalOption("autosave").(float64); a > 0 {
		config.SetAutoTime(a)
	}

	screen.Events = make(chan tcell.Event)

	util.Sigterm = make(chan os.Signal, 1)
	sighup = make(chan os.Signal, 1)
	signal.Notify(util.Sigterm, syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGABRT)
	signal.Notify(sighup, syscall.SIGHUP)

	timerChan = make(chan func())

	// loop de polling de eventos da tcell
	go func() {
		for {
			screen.Lock()
			e := screen.Screen.PollEvent()
			screen.Unlock()
			if e != nil {
				screen.Events <- e
			}
		}
	}()

	// drena desenho pendente antes de iniciar
	for len(screen.DrawChan()) > 0 {
		<-screen.DrawChan()
	}

	// espera primeiro resize
	select {
	case event := <-screen.Events:
		action.Tabs.HandleEvent(event)
	case <-time.After(10 * time.Millisecond):
	}

	for {
		DoEvent()
	}
}

// DoEvent runs the main action loop of the editor
func DoEvent() {
	var event tcell.Event

	// --- desenho ---
	screen.Screen.Fill(' ', config.DefStyle)
	screen.Screen.HideCursor()
	action.Tabs.Display()
	for _, ep := range action.MainTab().Panes {
		ep.Display()
	}
	action.MainTab().Display()
	action.InfoBar.Display()

	// overlays (menu / formulário) por cima
	helMenuDraw()
	helFormDraw()

	screen.Screen.Show()

	// --- espera por algo acontecer ---
	select {
	case f := <-shell.Jobs:
		f.Function(f.Output, f.Args)
	case <-config.Autosave:
		for _, b := range buffer.OpenBuffers {
			b.AutoSave()
		}
	case <-shell.CloseTerms:
		action.Tabs.CloseTerms()
	case event = <-screen.Events:
	case <-screen.DrawChan():
		for len(screen.DrawChan()) > 0 {
			<-screen.DrawChan()
		}
	case f := <-timerChan:
		f()
	case <-sighup:
		exit(0)
	case <-util.Sigterm:
		exit(0)
	}

	// erros da tcell
	if e, ok := event.(*tcell.EventError); ok {
		log.Println("tcell event error: ", e.Error())
		if e.Err() == io.EOF {
			exit(0)
		}
		return
	}

	if event == nil {
		return
	}

	// ============================
	// 1) FORM (consome teclas)
	// ============================
	if ev, ok := event.(*tcell.EventKey); ok {
		// --- FORM: atalhos para abrir e consumo das teclas ---
		// 1) Alt/Meta + F
		if (ev.Modifiers()&tcell.ModAlt) != 0 && (ev.Rune() == 'f' || ev.Rune() == 'F') {
			helFormOpen()
			return // consome: evita cair no binding padrão de busca por regex
		}

		// 2) Esc seguido de 'f' em até 200ms (muitos terminais codificam Alt como ESC + tecla)
		if ev.Key() == tcell.KeyEsc {
			helLastEscForm = time.Now()
		} else if (ev.Rune() == 'f' || ev.Rune() == 'F') && time.Since(helLastEscForm) < 200*time.Millisecond {
			helFormOpen()
			helLastEscForm = time.Time{}
			return // consome
		}

		// 3) Option+f no macOS que envia o caractere 'ƒ'
		if ev.Rune() == 'ƒ' { // U+0192
			helFormOpen()
			return // consome (senão o 'ƒ' cai no buffer)
		}

		// Se o formulário já está ativo, todas as teclas vão para ele
		if formActive {
			if helFormHandleKey(ev) {
				return // consome para não vazar para o editor
			}
		}

		if formActive {
			if helFormHandleKey(ev) {
				return
			}
		}
		// atalho: Alt-F abre o formulário
		if (ev.Modifiers()&tcell.ModAlt) != 0 && (ev.Rune() == 'f' || ev.Rune() == 'F') {
			helFormOpen()
			return
		}
	}

	// ============================
	// 2) MENU (consome teclas)
	// ============================
	if ev, ok := event.(*tcell.EventKey); ok {
		// se já está aberto, delega e consome
		if helMenuActive {
			if helMenuHandleKey(ev) {
				return
			}
		}
		// abrir com Alt/Meta + M
		if (ev.Modifiers()&tcell.ModAlt) != 0 && (ev.Rune() == 'm' || ev.Rune() == 'M') {
			helMenuOpen()
			return
		}
		// Esc seguido de 'm' (<=200ms)
		if ev.Key() == tcell.KeyEsc {
			helLastEsc = time.Now()
		} else if (ev.Rune() == 'm' || ev.Rune() == 'M') && time.Since(helLastEsc) < 200*time.Millisecond {
			helMenuOpen()
			helLastEsc = time.Time{}
			return
		}
		// Option+m que vira 'µ' no macOS
		if ev.Rune() == 'µ' {
			helMenuOpen()
			return
		}
	}

	// ============================
	// 3) PASTE (normaliza colagem)
	// ============================
	if ev, ok := event.(*tcell.EventPaste); ok {
		txt := strings.ReplaceAll(ev.Text(), "\r", "")
		lines := strings.Split(txt, "\n")
		for i := 1; i < len(lines); i++ {
			// remove UMA tab no início de cada linha após a primeira
			if len(lines[i]) > 0 && lines[i][0] == '\t' {
				lines[i] = lines[i][1:]
			}
		}
		cleaned := strings.Join(lines, "\n")
		event = tcell.NewEventPaste(cleaned, "")
	}

	// ============================
	// 4) roteamento normal
	// ============================
	if _, resize := event.(*tcell.EventResize); resize {
		action.InfoBar.HandleEvent(event)
		action.Tabs.HandleEvent(event)
	} else if action.InfoBar.HasPrompt {
		action.InfoBar.HandleEvent(event)
	} else {
		action.Tabs.HandleEvent(event)
	}

	// hooks de plugins
	if err := config.RunPluginFn("onAnyEvent"); err != nil {
		screen.TermMessage(err)
	}
}
