package runner

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rivo/tview"
	"github.com/santiago-labs/telophasecli/resource"
)

var tuiIndex atomic.Int64
var tuiLock sync.Mutex

type tui struct {
	tails map[string]*func() string
	index map[string]int
	list  *tview.List
	main  *tview.TextView
	app   *tview.Application
	tv    *tview.TextView
	files map[string]io.Writer
}

func NewTUI() ConsoleUI {
	app := tview.NewApplication()
	tv := tview.NewTextView().SetDynamicColors(true)

	main := tv.
		SetTextAlign(tview.AlignLeft).SetScrollable(true).
		SetChangedFunc(func() {
			tv.ScrollToEnd()
			app.Draw()
		}).SetText("Starting Telophase...")

	return &tui{
		list:  tview.NewList(),
		app:   app,
		main:  main,
		tv:    tv,
		index: make(map[string]int),
		tails: make(map[string]*func() string),
		files: make(map[string]io.Writer),
	}
}

func (t *tui) accountID(acct resource.Account) string {
	var acctId = acct.ID()
	if acctId == "" {
		acctId = fmt.Sprintf("Not yet provisioned (email: %s)", acct.Email)
	}

	return acctId
}

func (t *tui) createIfNotExists(acct resource.Account) {
	acctId := t.accountID(acct)

	tuiLock.Lock()
	defer tuiLock.Unlock()
	if _, ok := t.tails[acctId]; ok {
		return
	}

	idx := len(t.index)
	t.index[acctId] = idx

	t.list.AddItem(acctId, acct.AccountName, runeIndex(idx), func() {
		currText := *t.tails[acctId]
		// And we want to call this on repeat
		tuiIndex.Swap(int64(idx))
		tuiLock.Lock()
		defer tuiLock.Unlock()
		t.main.SetText(tview.TranslateANSI(currText()))
	})

	file, err := os.CreateTemp("/tmp", acctId)
	if err != nil {
		panic(err)
	}

	setter := func() string {
		bytes, err := os.ReadFile(file.Name())
		if err != nil {
			fmt.Printf("ERR: %s \n", err)
			return ""
		}

		return string(bytes)
	}

	t.tails[acctId] = &setter
	t.files[acctId] = file

	t.app.Draw()
}

func (t *tui) RunCmd(cmd *exec.Cmd, acct resource.Account) error {
	t.createIfNotExists(acct)

	acctId := t.accountID(acct)
	cmd.Stderr = t.files[acctId]
	cmd.Stdout = t.files[acctId]

	if err := cmd.Start(); err != nil {
		return err
	}

	if err := cmd.Wait(); err != nil {
		return err
	}
	return nil
}

func (t *tui) Start() {
	t.list.AddItem("Quit", "Press to exit", 'q', func() {
		t.app.Stop()
	})

	startScreen := func() string { return "Starting Telophase..." }
	t.tails["quit"] = &startScreen
	t.index["quit"] = 0

	tuiIndex.Swap(0)

	go t.liveTextSetter()

	grid := tview.NewGrid().
		SetColumns(-1, -3).
		SetRows(-1).
		SetBorders(true)

	// Layout for screens wider than 100 cells.
	grid.AddItem(t.list, 0, 0, 1, 1, 0, 100, false).
		AddItem(t.main, 0, 1, 1, 1, 0, 100, false)

	err := t.app.SetRoot(grid, true).SetFocus(t.list).Run()
	if err != nil {
		panic(err)
	}
}

func (t tui) Print(msg string, acct resource.Account) {
	t.createIfNotExists(acct)
	acctId := t.accountID(acct)

	fmt.Fprintf(t.files[acctId], "%s\n", msg)
}

func runeIndex(i int) rune {
	j := 0
	for r := 'a'; r <= 'p'; r++ {
		if j == i {
			return r
		}
		j++
	}

	return 'z'
}

// liveTextSetter updates the current tui view with the current tail's text.
func (t *tui) liveTextSetter() {
	defer func() {
		if r := recover(); r != nil {
			t.app.Stop()
		}
	}()
	for {
		func() {
			time.Sleep(200 * time.Millisecond)
			tuiLock.Lock()
			defer tuiLock.Unlock()
			var tailfunc func() string
			for key, val := range t.index {
				if int64(val) == tuiIndex.Load() {
					tailfunc = *t.tails[key]
				}
			}

			curr := t.tv.GetText(true)
			newText := tailfunc()
			if newText != curr && newText != "" {
				t.tv.SetText(tview.TranslateANSI(tailfunc()))
			}
		}()
	}
}
