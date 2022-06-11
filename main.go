package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	"github.com/aws/smithy-go"
	"github.com/gdamore/tcell/v2"
	"github.com/jessevdk/go-flags"
	"github.com/mindriot101/cflivestatus/fetcher"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var defStyle = tcell.StyleDefault.Background(tcell.ColorReset).Foreground(tcell.ColorReset)
var okStyle = tcell.StyleDefault.Background(tcell.ColorReset).Foreground(tcell.ColorGreen)
var updatingStyle = tcell.StyleDefault.Background(tcell.ColorReset).Foreground(tcell.ColorBlue)
var failedStyle = tcell.StyleDefault.Background(tcell.ColorReset).Foreground(tcell.ColorRed)

type Screen struct {
	s *tcell.Screen
}

func NewScreen() (*Screen, error) {
	s, err := tcell.NewScreen()
	if err != nil {
		return nil, fmt.Errorf("creating screen: %w", err)
	}
	if err := s.Init(); err != nil {
		return nil, fmt.Errorf("initialising screen: %w", err)
	}
	s.SetStyle(defStyle)
	s.Clear()

	return &Screen{s: &s}, nil
}

func (s *Screen) write(line int, style tcell.Style, format string, args ...interface{}) {
	row := line
	col := 0
	text := fmt.Sprintf(format, args...)
	runes := []rune(text)
	x2 := col + len(runes)
	for _, r := range runes {
		(*s.s).SetContent(col, row, r, nil, style)
		col++
		if col > x2 {
			row++
			col = 0
		}
		if row > line {
			break
		}
	}
}

func (s *Screen) Quit() {
	(*s.s).Fini()
	os.Exit(0)
}

func (s *Screen) show() {
	(*s.s).Show()
}

func (s *Screen) PollEvent() tcell.Event {
	return (*s.s).PollEvent()
}

func (s *Screen) Sync() {
	(*s.s).Sync()
}

func (s *Screen) clear() {
	(*s.s).Clear()
}

// sort interface
type byName []fetcher.StackResource

func (n byName) Len() int           { return len(n) }
func (n byName) Swap(i, j int)      { n[i], n[j] = n[j], n[i] }
func (n byName) Less(i, j int) bool { return n[i].Resource < n[j].Resource }

func longestResourceName(res []fetcher.StackResource) int {
	if len(res) == 0 {
		return 0
	}
	m := len(res[0].Resource)
	for _, r := range res {
		if len(r.Resource) > m {
			m = len(r.Resource)
		}
	}
	return m
}

func (s *Screen) Render(statuses []fetcher.StackResource) {
	s.clear()
	i := 0
	now := time.Now()
	s.write(i, defStyle, "%s", now.Format(time.RFC1123Z))
	i++

	sort.Sort(byName(statuses))
	nameLength := longestResourceName(statuses)

	for _, r := range statuses {
		if r.Reason != "" {
			fs := fmt.Sprintf("%%%ds: %%s (%%s)", nameLength)
			s.write(i, resourceStyle(r.Status), fs, r.Resource, r.Status, r.Reason)
		} else {
			fs := fmt.Sprintf("%%%ds: %%s", nameLength)
			s.write(i, resourceStyle(r.Status), fs, r.Resource, r.Status)
		}
		i++
	}
	s.show()
}

func resourceStyle(status types.ResourceStatus) tcell.Style {
	var style tcell.Style
	switch status {
	case types.ResourceStatusCreateComplete,
		types.ResourceStatusUpdateComplete,
		types.ResourceStatusDeleteComplete,
		types.ResourceStatusRollbackComplete:
		style = okStyle
	case types.ResourceStatusCreateInProgress,
		types.ResourceStatusUpdateInProgress,
		types.ResourceStatusDeleteInProgress,
		types.ResourceStatusRollbackInProgress:
		style = updatingStyle
	case types.ResourceStatusCreateFailed,
		types.ResourceStatusUpdateFailed,
		types.ResourceStatusDeleteFailed,
		types.ResourceStatusRollbackFailed:
		style = failedStyle
	default:
		style = defStyle
	}
	return style
}

func main() {

	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{
		Out: os.Stderr,
	})

	ctx := context.TODO()

	var opts struct {
		SleepTime time.Duration `short:"s" long:"sleep-time" required:"no" default:"0"`
		Verbose   []bool        `short:"v" long:"verbose" description:"Print verbose logging output"`
		Args      struct {
			Name string `required:"yes" positional-arg-name:"stack-name"`
		} `positional-args:"yes" required:"yes"`
	}

	_, err := flags.Parse(&opts)
	if err != nil {
		os.Exit(1)
	}
	switch len(opts.Verbose) {
	case 0:
		zerolog.SetGlobalLevel(zerolog.WarnLevel)
	case 1:
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	default:
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}

	log.Debug().Msgf("%s starting", os.Args[0])
	log.Debug().Interface("opts", opts).Msg("parsed command line options")

	// TODO: update default region
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		log.Err(err).Msg("error loading default config")
	}

	svc := cloudformation.NewFromConfig(cfg)
	f := fetcher.New(opts.Args.Name, svc)

	// update resources goroutine
	eventsCh := make(chan []fetcher.StackResource)
	go func() {
		for {
			resources, err := f.Fetch(ctx)
			if err != nil {
				if handleFetchResourceError(opts.Args.Name, err) {
					log.Fatal().Err(err).Msg("a fatal error occurred")
				}

				log.Warn().Err(err).Msg("error when polling stack resources")
				time.Sleep(opts.SleepTime)
				continue
			}
			eventsCh <- resources

			time.Sleep(opts.SleepTime)
		}
	}()

	<-eventsCh

	screen, err := NewScreen()
	if err != nil {
		panic(err)
	}

	// background goroutine that sends events to the main render loop
	done := make(chan struct{})
	go func() {
		for {
			ev := screen.PollEvent()
			switch ev := ev.(type) {
			case *tcell.EventResize:
				screen.Sync()
			case *tcell.EventKey:
				switch ev.Key() {
				case tcell.KeyCtrlC, tcell.KeyEscape:
					close(done)
					return
				case tcell.KeyCtrlL:
					screen.Sync()
				}
			}
		}
	}()

	for {
		select {
		case <-done:
			screen.Quit()
			return
		case res := <-eventsCh:
			screen.Render(res)
		}
	}
}

// handleFetchResourceError returns whether the loop should break or not,
// given the error supplied
func handleFetchResourceError(name string, err error) bool {
	if err == nil {
		return false
	}
	var oe *smithy.GenericAPIError
	if errors.As(err, &oe) {
		if oe.Message == fmt.Sprintf("Stack with id %s does not exist", name) {
			return true
		}
	}
	return true
}
