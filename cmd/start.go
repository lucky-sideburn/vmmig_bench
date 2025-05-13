/*
Copyright © 2025 Sourcesense <eugenio.marzo@sourcesense.com>
*/

package cmd

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/spf13/cobra"
)

var (
	token        string
	serverURL    string
	namespaces   string
	sleepSeconds int

	// Prometheus metrics
	vmCountMetric = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "virtual_machine_count_total",
		Help: "Total number of virtual machines in the namespace",
	}, []string{"namespace"})

	vmStatusMetric = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "virtual_machine_status",
		Help: "Status of a virtual machine in the namespace",
	}, []string{"namespace", "vm_name", "status"})

	failedMigrationsMetric = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "failed_migrations_total",
		Help: "Total number of failed migrations per namespace",
	}, []string{"namespace"})

	migrationTimeMetric = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "virtual_machine_migration_time_seconds",
		Help: "Time taken for virtual machine migrations in the namespace",
	}, []string{"namespace", "vm_name"})
)

func init() {
	sleepSeconds = 15
	// Register Prometheus metrics
	prometheus.MustRegister(vmCountMetric)
	prometheus.MustRegister(vmStatusMetric)
	prometheus.MustRegister(failedMigrationsMetric)
	prometheus.MustRegister(migrationTimeMetric)
}

// startCmd represents the start command
var startCmd = &cobra.Command{
	Use:   "start",
	Short: "A Prometheus exporter designed to extract metrics for OpenShift Virtualization (KubeVirt)",
	Long:  `...`,
	Run: func(cmd *cobra.Command, args []string) {
		if token == "" || serverURL == "" || namespaces == "" {
			fmt.Println("Error: --token, --server-url, and --namespaces are required parameters")
			cmd.Usage()
			os.Exit(1)
		}

		namespaceList := strings.Split(namespaces, ",")

		fmt.Println("========================================")
		fmt.Println("          vmmig_bench Exporter          ")
		fmt.Println("          Powered by DevOpsTribe.it     ")
		fmt.Println("========================================")
		fmt.Printf("start called with --token=*** --server-url=%s --namespaces=%v\n", serverURL, namespaceList)
		fmt.Println("Starting Prometheus exporter...")

		// Prometheus exporter initialization
		fmt.Println("Initializing Prometheus exporter...")
		registry := prometheus.NewRegistry()

		if err := registry.Register(vmStatusMetric); err != nil {
			if are, ok := err.(prometheus.AlreadyRegisteredError); ok {
				vmStatusMetric = are.ExistingCollector.(*prometheus.GaugeVec)
			} else {
				fmt.Printf("Error registering vmStatusMetric: %v\n", err)
				os.Exit(1)
			}
		}

		if err := registry.Register(failedMigrationsMetric); err != nil {
			if are, ok := err.(prometheus.AlreadyRegisteredError); ok {
				failedMigrationsMetric = are.ExistingCollector.(*prometheus.CounterVec)
			} else {
				fmt.Printf("Error registering failedMigrationsMetric: %v\n", err)
				os.Exit(1)
			}
		}

		if err := registry.Register(migrationTimeMetric); err != nil {
			if are, ok := err.(prometheus.AlreadyRegisteredError); ok {
				migrationTimeMetric = are.ExistingCollector.(*prometheus.GaugeVec)
			} else {
				fmt.Printf("Error registering migrationTimeMetric: %v\n", err)
				os.Exit(1)
			}
		}

		if err := registry.Register(vmCountMetric); err != nil {
			if are, ok := err.(prometheus.AlreadyRegisteredError); ok {
				vmCountMetric = are.ExistingCollector.(*prometheus.GaugeVec)
			} else {
				fmt.Printf("Error registering vmCountMetric: %v\n", err)
				os.Exit(1)
			}
		}

		// Start a background thread to update the VM count for each namespace
		go func() {
			for {
				for _, namespace := range namespaceList {
					exportVirtualMachineCount(serverURL, token, namespace)
					exportVirtualMachineNamesAndStatuses(serverURL, token, namespace)
				}
				exportVirtualMachineMigrationTime(serverURL, token)

				time.Sleep(time.Duration(sleepSeconds) * time.Second) // Update every sleepSeconds
			}
		}()

		http.Handle("/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))
		fmt.Println("Prometheus exporter initialized successfully.")

		// Start Prometheus exporter
		fmt.Println("Starting Prometheus exporter on :8080")
		if err := http.ListenAndServe(":8080", nil); err != nil {
			fmt.Printf("Error starting Prometheus exporter: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(startCmd)

	startCmd.Flags().StringVar(&token, "token", "", "Authentication token (required)")
	startCmd.Flags().StringVar(&serverURL, "server-url", "", "Server URL (required)")
	startCmd.Flags().StringVar(&namespaces, "namespaces", "", "Comma-separated list of namespaces (required)")
}

func exportVirtualMachineMigrationTime(serverURL, token string) (bool, error) {
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}

	req, err := http.NewRequest("GET", fmt.Sprintf("%s/apis/forklift.konveyor.io/v1beta1/migrations", serverURL), nil)

	fmt.Printf("Request URL: %s\n", req.URL.String())

	if err != nil {
		fmt.Printf("Failed to create request for migrations endpoint: %v\n", err)
		return false, err
	}

	// Create a custom HTTP client that ignores TLS verification
	client := &http.Client{}

	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	fmt.Printf("Response status code when calling API for migrations: %d\n", resp.StatusCode)

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Parse the response to extract migration times
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, err
	}

	var migrations struct {
		Items []struct {
			Status struct {
				Namespace string `json:"namespace"`
				Vms       []struct {
					Name      string `json:"name"`
					Started   string `json:"started"`
					Completed string `json:"completed"`
				} `json:"vms"`
			} `json:"status"`
		} `json:"items"`
	}

	if err := json.Unmarshal(body, &migrations); err != nil {
		return false, err
	}

	for _, migration := range migrations.Items {
		for _, vm := range migration.Status.Vms {
			started, err := time.Parse(time.RFC3339, vm.Started)
			if err != nil {
				fmt.Printf("Error parsing start time for VM %s: %v\n", vm.Name, err)
				continue
			}
			fmt.Printf("VM %s started at: %s\n", vm.Name, started)
			completed, err := time.Parse(time.RFC3339, vm.Completed)
			if err != nil {
				fmt.Printf("Error parsing completion time for VM %s: %v\n", vm.Name, err)
				continue
			}
			fmt.Printf("VM %s completed at: %s\n", vm.Name, completed)

			duration := completed.Sub(started).Seconds()
			// if duration > 60 {
			// 	duration = duration / 60
			// 	fmt.Printf("VM %s migration duration: %f minutes\n", vm.Name, duration)
			// 	migrationTimeMetric.WithLabelValues(migration.Status.Namespace, vm.Name).Set(duration)
			// } else {
			// 	migrationTimeMetric.WithLabelValues(migration.Status.Namespace).Set(duration)
			// }

			fmt.Printf("VM %s migration duration: %f seconds\n", vm.Name, duration)
			migrationTimeMetric.WithLabelValues(migration.Status.Namespace, vm.Name).Set(duration)

		}
	}

	return true, nil

}

func exportVirtualMachineNamesAndStatuses(serverURL, token, namespace string) (map[string]string, error) {
	fmt.Printf("Fetching virtual machine names and statuses for namespace %s...\n", namespace)
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}

	req, err := http.NewRequest("GET", fmt.Sprintf("%s/apis/kubevirt.io/v1/namespaces/%s/virtualmachines", serverURL, namespace), nil)
	fmt.Printf("Request URL: %s\n", req.URL.String())
	if err != nil {
		return nil, err
	}

	// Create a custom HTTP client that ignores TLS verification
	client := &http.Client{}

	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Parse the response to extract VM names and statuses
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var vmList struct {
		Items []struct {
			Metadata struct {
				Name string `json:"name"`
			} `json:"metadata"`
			Status struct {
				PrintableStatus string `json:"printableStatus"`
			} `json:"status"`
		} `json:"items"`
	}

	if err := json.Unmarshal(body, &vmList); err != nil {
		return nil, err
	}

	vmNamesAndStatuses := make(map[string]string)
	for _, vm := range vmList.Items {
		vmNamesAndStatuses[vm.Metadata.Name] = vm.Status.PrintableStatus

		// Expose VM status as a Prometheus metric
		vmStatusMetric.WithLabelValues(namespace, vm.Metadata.Name, vm.Status.PrintableStatus).Set(1) // Set to 1 to indicate the VM exists with the given status
	}

	return vmNamesAndStatuses, nil
}

// getVirtualMachineCount fetches the number of virtual machines from OpenShift Virtualization
func exportVirtualMachineCount(serverURL, token, namespace string) (int, error) {
	// Simulate API call to OpenShift Virtualization
	fmt.Printf("Fetching virtual machine count for namespace %s...\n", namespace)
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}

	req, err := http.NewRequest("GET", fmt.Sprintf("%s/apis/kubevirt.io/v1/namespaces/%s/virtualmachines", serverURL, namespace), nil)
	fmt.Printf("Request URL: %s\n", req.URL.String())
	if err != nil {
		return 0, err
	}

	// Create a custom HTTP client that ignores TLS verification
	client := &http.Client{}

	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Parse the response to count the virtual machines
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	var vmList struct {
		Items []struct{} `json:"items"`
	}

	if err := json.Unmarshal(body, &vmList); err != nil {
		return 0, err
	}

	// Return the count of virtual machines
	count := len(vmList.Items)

	vmCountMetric.WithLabelValues(namespace).Set(float64(count))

	return count, nil
}
