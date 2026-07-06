// Command soak stress-tests the write path against a REAL Things Cloud
// account by driving the things-cli binary through many create/edit/move/
// complete/trash cycles, including identifiers deliberately built from
// UUIDs with leading zero bytes — the ~1/256 case that used to corrupt
// sync histories before the canonical Base58 fix.
//
// It then verifies every write landed by reading the account back through
// the sync engine, and finally trashes and purges everything it created.
//
// SAFETY: this writes to a live account. Point it ONLY at a throwaway test
// account. It refuses to run without an explicit confirmation env var.
//
//	THINGS_USERNAME=test@example.com \
//	THINGS_PASSWORD=... \
//	THINGS_SOAK_CONFIRM=yes-throwaway-account \
//	go run ./cmd/soak --cycles 50
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	things "github.com/arthursoares/things-cloud-sdk"
	"github.com/arthursoares/things-cloud-sdk/sync"
	"github.com/google/uuid"
)

type stats struct {
	creates      int
	forcedLeadZ  int
	edits        int
	moves        int
	completes    int
	trashed      int
	purged       int
	rejections   int
	verifyChecks int
	verifyFails  int
}

func main() {
	cycles := flag.Int("cycles", 25, "number of create/edit/move/complete/trash cycles")
	cliPath := flag.String("cli", "", "path to things-cli binary (built automatically if empty)")
	keep := flag.Bool("keep", false, "skip cleanup (leave created items in the account)")
	flag.Parse()

	if os.Getenv("THINGS_SOAK_CONFIRM") != "yes-throwaway-account" {
		fatal("refusing to run: set THINGS_SOAK_CONFIRM=yes-throwaway-account and use a DISPOSABLE test account only")
	}
	user := requireEnv("THINGS_USERNAME")
	requireEnv("THINGS_PASSWORD")

	cli := *cliPath
	if cli == "" {
		cli = buildCLI()
	}

	fmt.Printf("Soak test → account %s, %d cycles\n", user, *cycles)
	fmt.Println("This writes to a LIVE account. Ctrl-C now if it is not disposable.")
	time.Sleep(3 * time.Second)

	var st stats
	created := runCycles(cli, *cycles, &st)

	verify(created, &st)

	if !*keep {
		cleanup(cli, created, &st)
	}

	fmt.Println("\n=== Soak summary ===")
	fmt.Printf("creates:        %d (of which forced leading-zero UUID: %d)\n", st.creates, st.forcedLeadZ)
	fmt.Printf("edits:          %d\n", st.edits)
	fmt.Printf("moves:          %d\n", st.moves)
	fmt.Printf("completes:      %d\n", st.completes)
	fmt.Printf("trashed/purged: %d / %d\n", st.trashed, st.purged)
	fmt.Printf("Write rejections (unexpected): %d\n", st.rejections)
	fmt.Printf("verification checks: %d, failures: %d\n", st.verifyChecks, st.verifyFails)

	if st.rejections > 0 || st.verifyFails > 0 {
		fmt.Println("\nRESULT: FAIL — the account may hold data Things.app cannot decode. Inspect before syncing a real client.")
		os.Exit(1)
	}
	fmt.Println("\nRESULT: PASS — every write was accepted and verified. Now open Things.app on this account and confirm it syncs cleanly.")
}

type createdItem struct {
	uuid  string
	title string
	kind  string // "task" | "project" | "area" | "tag"
}

// forcedLeadingZeroUUID returns a canonical Base58 identifier whose
// underlying UUID starts with one or two zero bytes — the historical
// corruption trigger. With the fix this is a perfectly valid identifier.
func forcedLeadingZeroUUID(two bool) string {
	u := uuid.New()
	u[0] = 0x00
	if two {
		u[1] = 0x00
	}
	return things.EncodeUUID(u)
}

func runCycles(cli string, cycles int, st *stats) []createdItem {
	var created []createdItem

	for i := 0; i < cycles; i++ {
		// Project (structural — must never land in inbox)
		projID := things.NewUUID()
		runCLI(cli, st, "create", fmt.Sprintf("Soak project %d", i), "--type", "project", "--uuid", projID)
		st.creates++
		created = append(created, createdItem{projID, fmt.Sprintf("Soak project %d", i), "project"})

		// Heading under the project, explicitly requested inbox to prove
		// the structural st=1 override holds on the real server.
		headID := things.NewUUID()
		runCLI(cli, st, "create", fmt.Sprintf("Soak heading %d", i), "--type", "heading", "--project", projID, "--when", "inbox", "--uuid", headID)
		st.creates++
		created = append(created, createdItem{headID, fmt.Sprintf("Soak heading %d", i), "heading"})

		// A batch of tasks under the heading. Every 4th task is forced to
		// use a leading-zero-byte UUID.
		for j := 0; j < 4; j++ {
			var taskID string
			if j == 0 {
				taskID = forcedLeadingZeroUUID(j == 0 && i%8 == 0)
				st.forcedLeadZ++
			} else {
				taskID = things.NewUUID()
			}
			title := fmt.Sprintf("Soak task %d.%d", i, j)
			runCLI(cli, st, "create", title, "--heading", headID, "--project", projID, "--uuid", taskID)
			st.creates++
			created = append(created, createdItem{taskID, title, "task"})

			// Edit, then complete the first, move the second to today.
			runCLI(cli, st, "edit", taskID, "--note", fmt.Sprintf("soak note %d.%d", i, j))
			st.edits++
			switch j {
			case 0:
				runCLI(cli, st, "complete", taskID)
				st.completes++
			case 1:
				runCLI(cli, st, "move-to-today", taskID)
				st.moves++
			}
		}
	}
	return created
}

func verify(created []createdItem, st *stats) {
	fmt.Println("\nVerifying via sync engine …")
	client := things.New(things.APIEndpoint, os.Getenv("THINGS_USERNAME"), os.Getenv("THINGS_PASSWORD"))
	dbPath := filepath.Join(os.TempDir(), "soak-verify.db")
	_ = os.Remove(dbPath)
	syncer, err := sync.Open(dbPath, client)
	if err != nil {
		fatal("open verify db: " + err.Error())
	}
	defer syncer.Close()

	if _, err := syncer.Sync(); err != nil {
		fatal("verify sync failed: " + err.Error())
	}
	// Tasks, projects, and headings all live in the tasks table but are
	// returned by different queries (type 0/1/2), so gather all three.
	state := syncer.State()
	opts := sync.QueryOpts{IncludeCompleted: true, IncludeTrashed: true}
	present := map[string]bool{}
	for _, q := range []func(sync.QueryOpts) ([]*things.Task, error){
		state.AllTasks, state.AllProjects, state.AllHeadings,
	} {
		items, err := q(opts)
		if err != nil {
			fatal("query state: " + err.Error())
		}
		for _, it := range items {
			present[it.UUID] = true
		}
	}
	for _, c := range created {
		if c.kind == "area" || c.kind == "tag" {
			continue
		}
		st.verifyChecks++
		if !present[c.uuid] {
			st.verifyFails++
			fmt.Printf("  MISSING after sync: %s %q (%s)\n", c.kind, c.title, c.uuid)
		}
	}
	fmt.Printf("  %d/%d created task/project items found in synced state\n", st.verifyChecks-st.verifyFails, st.verifyChecks)
}

func cleanup(cli string, created []createdItem, st *stats) {
	fmt.Println("\nCleaning up (trash + purge) …")
	// Trash then purge in reverse creation order so children go before parents.
	for i := len(created) - 1; i >= 0; i-- {
		c := created[i]
		if c.kind == "area" || c.kind == "tag" {
			continue // areas/tags are not created by this soak yet
		}
		if runCLIQuiet(cli, "trash", c.uuid) {
			st.trashed++
		}
		if runCLIQuiet(cli, "purge", c.uuid) {
			st.purged++
		}
	}
}

// --- process helpers ---

func runCLI(cli string, st *stats, args ...string) map[string]string {
	out, err := exec.Command(cli, args...).CombinedOutput()
	if err != nil {
		st.rejections++
		fmt.Printf("  REJECTED: %s → %v\n    %s\n", strings.Join(args, " "), err, bytes.TrimSpace(out))
		return nil
	}
	var res map[string]string
	_ = json.Unmarshal(out, &res)
	return res
}

func runCLIQuiet(cli string, args ...string) bool {
	return exec.Command(cli, args...).Run() == nil
}

func buildCLI() string {
	fmt.Println("Building things-cli …")
	bin := filepath.Join(os.TempDir(), "things-cli-soak")
	out, err := exec.Command("go", "build", "-o", bin, "./cmd/things-cli").CombinedOutput()
	if err != nil {
		fatal(fmt.Sprintf("building things-cli: %v\n%s", err, out))
	}
	return bin
}

func requireEnv(k string) string {
	v := os.Getenv(k)
	if v == "" {
		fatal(k + " is required")
	}
	return v
}

func fatal(msg string) {
	fmt.Fprintln(os.Stderr, "soak: "+msg)
	os.Exit(1)
}

var _ = bufio.NewReader // reserved for future interactive confirm
