/*
Copyright © 2025 Sourcesense <eugenio.marzo@sourcesense.com>
*/

package cmd

import (
	"fmt"

	"github.com/rivo/tview"
	"github.com/spf13/cobra"
)

// configCmd represents the config command
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "A Prometheus exporter designed to extract metrics for OpenShift Virtualization (KubeVirt)",
	Long:  `...`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("config called")
		startGUI()
	},
}

func init() {
	rootCmd.AddCommand(configCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// configCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// configCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

func startGUI() {
	app := tview.NewApplication()

	textView := tview.NewTextView().
		SetText("Hello, world!").
		SetTextAlign(tview.AlignCenter).
		SetDynamicColors(true)

	if err := app.SetRoot(textView, true).Run(); err != nil {
		panic(err)
	}
}
