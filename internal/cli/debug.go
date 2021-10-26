package cli

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/alexander-yu/stream/minmax"
	"github.com/alexander-yu/stream/moment"
	"github.com/alexander-yu/stream/quantile"
	"github.com/mattn/go-isatty"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/epinio/epinio/internal/cli/usercmd"
)

var ()

func init() {
	CmdDebug.AddCommand(CmdDebugTTY)
	CmdDebug.AddCommand(CmdDebugLoad)
}

// CmdDebug implements the command: epinio debug
var CmdDebug = &cobra.Command{
	Hidden:        true,
	Use:           "debug",
	Short:         "Dev Tools",
	Long:          `Developer Tools. Hidden From Regular User.`,
	SilenceErrors: true,
	SilenceUsage:  true,
	Args:          cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := cmd.Usage(); err != nil {
			return err
		}
		return fmt.Errorf(`Unknown method "%s"`, args[0])
	},
}

// CmdDebugTTY implements the command: epinio debug tty
var CmdDebugTTY = &cobra.Command{
	Use:   "tty",
	Short: "Running In a Terminal?",
	Long:  `Running In a Terminal?`,
	Args:  cobra.ExactArgs(0),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		if isatty.IsTerminal(os.Stdout.Fd()) {
			fmt.Println("Is Terminal")
		} else if isatty.IsCygwinTerminal(os.Stdout.Fd()) {
			fmt.Println("Is Cygwin/MSYS2 Terminal")
		} else {
			fmt.Println("Is Not Terminal")
		}
		return nil
	},
}

// CmdDebugLoad implements the command: epinio debug load
var CmdDebugLoad = &cobra.Command{
	Use:   "load count millis",
	Short: "Generate server load",
	Long:  `Generate server load and collect response statistics`,
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true

		count, err := strconv.Atoi(args[0])
		if err != nil {
			return errors.Wrap(err, "bad count")
		}
		millis, err := strconv.Atoi(args[1])
		if err != nil {
			return errors.Wrap(err, "bad millis")
		}

		client, err := usercmd.New()
		if err != nil {
			return errors.Wrap(err, "error initializing cli")
		}

		timing := make(chan time.Duration, 10000)

		// Load Engine
		// - naive, trivial: ping every X milliseconds

		// TODO:
		// - jitter (randomization)
		// - multiple streams (same load per stream)
		// - streams with different loads
		// - general user simulation driven by some distribution
		//   + distribution driven user creation/deletion (streams)

		go func(timing chan time.Duration) {
			// millis and count are from outer scope.
			delta := time.Duration(millis) * time.Millisecond

			// Two unmeasured initial calls as warmup
			client.API.Info() // nolint:errcheck // Result is irrelevant
			client.API.Info() // nolint:errcheck // Result is irrelevant

			for k := 0; k < count; k++ {
				start := time.Now()
				client.API.Info() // nolint:errcheck // Result is irrelevant
				timing <- time.Since(start)
				time.Sleep(delta)
			}
			close(timing)
		}(timing)

		// Receiver and statistics engine (streaming)

		quants, _ := quantile.NewGlobalQuantile()
		max := minmax.NewGlobalMax()
		min := minmax.NewGlobalMin()
		mean := moment.NewGlobalMean()
		stdv := moment.NewGlobalStd()
		moment.Init(mean) // nolint:errcheck
		moment.Init(stdv) // nolint:errcheck

		pings := 0
		for duration := range timing {
			pings++

			fmt.Fprintf(os.Stderr, "\r\033[2K%dms %d/%d", millis, count, pings)

			dmilli := float64(duration.Milliseconds())

			mean.Push(dmilli)   // nolint:errcheck
			stdv.Push(dmilli)   // nolint:errcheck
			min.Push(dmilli)    // nolint:errcheck
			max.Push(dmilli)    // nolint:errcheck
			quants.Push(dmilli) // nolint:errcheck
		}

		fmt.Fprintf(os.Stderr, "\r\033[2K")

		minv, _ := min.Value()        // nolint:errcheck
		maxv, _ := max.Value()        // nolint:errcheck
		avgv, _ := mean.Value()       // nolint:errcheck
		varv, _ := stdv.Value()       // nolint:errcheck
		tenv, _ := quants.Value(0.1)  // nolint:errcheck
		fifv, _ := quants.Value(0.5)  // nolint:errcheck
		ninv, _ := quants.Value(0.9)  // nolint:errcheck
		niiv, _ := quants.Value(0.99) // nolint:errcheck

		reqs := 1000 / millis

		fmt.Printf("pings___ millis req/s_ | min___     max___ | mean_____  stdvar_ | 10%%____ 50%%____ 90%%____ 99%%____\n")
		fmt.Printf("%8d %6d %6d | %6.0f ... %6.0f | %9.4f  %7.4f | %7.3f %7.3f %7.3f %7.3f\n",
			pings, millis, reqs, minv, maxv, avgv, varv, tenv, fifv, ninv, niiv)

		return nil
	},
}
