package runner

import (
	"fmt"
	"io"
	"io/ioutil"
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
		}).SetText("Starting CDK...")

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

func (t *tui) createIfNotExists(acct resource.Account) {
	var acctId = acct.ID()
	if acctId == "" {
		acctId = "Not yet provisioned"
	}

	tuiLock.Lock()
	defer tuiLock.Unlock()
	if _, ok := t.tails[acct.ID()]; ok {
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

	file, err := ioutil.TempFile("/tmp", acctId)
	if err != nil {
		panic(err)
	}

	setter := func() string {
		bytes, err := ioutil.ReadFile(file.Name())
		if err != nil {
			fmt.Printf("ERR: %s \n", err)
			return ""
		}

		return string(bytes)
	}

	t.tails[acct.ID()] = &setter
	t.files[acct.ID()] = file
}

func (t *tui) RunCmd(cmd *exec.Cmd, acct resource.Account) error {
	t.createIfNotExists(acct)
	cmd.Stderr = t.files[acct.ID()]
	cmd.Stdout = t.files[acct.ID()]

	if err := cmd.Start(); err != nil {
		return err
	}

	if err := cmd.Wait(); err != nil {
		return err
	}
	return nil
}

func (t *tui) PostProcess() {
	t.list.AddItem("Quit", "Press to exit", 'q', func() {
		t.app.Stop()
	})

	// Start index at 0 for the first account.
	tuiIndex.Swap(0)

	go t.liveTextSetter()

	grid := tview.NewGrid().
		SetColumns(-1, -3).
		SetRows(-1).
		SetBorders(true)

	// Layout for screens wider than 100 cells.
	grid.AddItem(t.list, 0, 0, 1, 1, 0, 100, false).
		AddItem(t.main, 0, 1, 1, 1, 0, 100, false)

	if err := t.app.SetRoot(grid, true).SetFocus(t.list).Run(); err != nil {
		panic(err)
	}

}

func (t tui) Print(msg string, acct resource.Account) {
	t.createIfNotExists(acct)
	fmt.Fprint(t.files[acct.ID()], msg)
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
