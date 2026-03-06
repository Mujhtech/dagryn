package graph

import (
	"fmt"
	"strings"

	"github.com/mujhtech/dagryn/internal/cli"
	"github.com/mujhtech/dagryn/pkg/dagryn/config"
	"github.com/mujhtech/dagryn/pkg/dagryn/dag"
	"github.com/mujhtech/dagryn/pkg/dagryn/scheduler"
	"github.com/spf13/cobra"
)

// NewCmd creates the graph command.
func NewCmd(flags *cli.Flags) *cobra.Command {
	return &cobra.Command{
		Use:   "graph",
		Short: "Visualize the task DAG",
		Long:  `Display an ASCII representation of the task dependency graph.`,
		Example: `  dagryn graph
  dagryn graph -c custom.toml`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGraph(cmd, args, flags)
		},
	}
}

func runGraph(cmd *cobra.Command, args []string, flags *cli.Flags) error {
	// Load config
	cfg, err := config.Parse(flags.CfgFile)
	if err != nil {
		return err
	}

	// Validate config
	if errors := config.Validate(cfg); len(errors) > 0 {
		return fmt.Errorf("configuration validation failed: %s", errors[0].Error())
	}

	// Convert to workflow
	workflow, err := cfg.ToWorkflow()
	if err != nil {
		return fmt.Errorf("failed to create workflow: %w", err)
	}

	// Get project root
	projectRoot, err := cli.GetProjectRoot()
	if err != nil {
		return fmt.Errorf("failed to get project root: %w", err)
	}

	// Create scheduler to get the graph
	sched, err := scheduler.New(workflow, projectRoot, scheduler.DefaultOptions())
	if err != nil {
		return err
	}

	graph := sched.GetGraph()
	plan, err := sched.GetExecutionPlan(nil)
	if err != nil {
		return err
	}

	// Print ASCII graph
	fmt.Println()
	fmt.Println("Task Dependency Graph")
	fmt.Println(strings.Repeat("═", 40))
	fmt.Println()

	printASCIIGraph(graph, plan)

	fmt.Println()
	fmt.Println(strings.Repeat("═", 40))
	fmt.Printf("Total: %d tasks\n", graph.Size())

	return nil
}

// printASCIIGraph prints an ASCII representation of the DAG.
func printASCIIGraph(g *dag.Graph, plan *dag.ExecutionPlan) {
	if plan.TotalTasks() == 0 {
		fmt.Println("  (no tasks)")
		return
	}

	// Find the maximum task name length for formatting
	maxLen := 0
	for _, level := range plan.Levels {
		for _, name := range level {
			if len(name) > maxLen {
				maxLen = len(name)
			}
		}
	}

	boxWidth := maxLen + 4
	if boxWidth < 10 {
		boxWidth = 10
	}

	// Print each level
	for i, level := range plan.Levels {
		// Calculate spacing for centering
		leftPad := ""

		// Print boxes for this level
		// Top border
		for j := range level {
			if j > 0 {
				fmt.Print("  ")
			}
			fmt.Print(leftPad)
			fmt.Print("┌" + strings.Repeat("─", boxWidth-2) + "┐")
		}
		fmt.Println()

		// Task name
		for j, name := range level {
			if j > 0 {
				fmt.Print("  ")
			}
			fmt.Print(leftPad)
			padding := boxWidth - 2 - len(name)
			leftSpace := padding / 2
			rightSpace := padding - leftSpace
			fmt.Printf("│%s%s%s│", strings.Repeat(" ", leftSpace), name, strings.Repeat(" ", rightSpace))
		}
		fmt.Println()

		// Bottom border
		for j := range level {
			if j > 0 {
				fmt.Print("  ")
			}
			fmt.Print(leftPad)
			fmt.Print("└" + strings.Repeat("─", boxWidth-2) + "┘")
		}
		fmt.Println()

		// Print connectors to next level if not last level
		if i < len(plan.Levels)-1 {
			nextLevel := plan.Levels[i+1]

			// Simple connector: just show arrows
			for j, name := range level {
				if j > 0 {
					fmt.Print("  ")
				}
				fmt.Print(leftPad)

				// Check if this task has dependents in the next level
				hasDependent := false
				for _, nextName := range nextLevel {
					deps := g.GetDependencies(nextName)
					for _, dep := range deps {
						if dep == name {
							hasDependent = true
							break
						}
					}
				}

				center := boxWidth / 2
				if hasDependent {
					fmt.Print(strings.Repeat(" ", center-1) + "│" + strings.Repeat(" ", boxWidth-center-1))
				} else {
					fmt.Print(strings.Repeat(" ", boxWidth))
				}
			}
			fmt.Println()

			// Arrow pointing down
			for j, name := range level {
				if j > 0 {
					fmt.Print("  ")
				}
				fmt.Print(leftPad)

				hasDependent := false
				for _, nextName := range nextLevel {
					deps := g.GetDependencies(nextName)
					for _, dep := range deps {
						if dep == name {
							hasDependent = true
							break
						}
					}
				}

				center := boxWidth / 2
				if hasDependent {
					fmt.Print(strings.Repeat(" ", center-1) + "▼" + strings.Repeat(" ", boxWidth-center-1))
				} else {
					fmt.Print(strings.Repeat(" ", boxWidth))
				}
			}
			fmt.Println()
		}
	}
}
