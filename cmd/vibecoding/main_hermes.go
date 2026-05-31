package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/startvibecoding/vibecoding/internal/cron"
	"github.com/startvibecoding/vibecoding/internal/hermes"
	"github.com/startvibecoding/vibecoding/internal/memory"
	"github.com/startvibecoding/vibecoding/internal/messaging/wechat"
)

// newHermesCommand builds the "hermes" command tree with all subcommands.
func newHermesCommand() *cobra.Command {
	var (
		flagPort       int
		flagWorkDir    string
		flagConfig     string
		flagProvider   string
		flagModel      string
		flagMultiAgent bool
		flagSandbox    bool
		flagDaemon     bool
		flagVerbose    bool
		flagDebug      bool
		flagForce      bool
	)

	hermesCmd := &cobra.Command{
		Use:   "hermes",
		Short: "Run the Hermes messaging gateway",
		Long:  "Start VibeCoding Hermes — a messaging gateway with WebSocket/HTTP API, WeChat, Feishu, and more.",
	}

	// --- start / stop / status ---

	startCmd := &cobra.Command{
		Use:   "start",
		Short: "Start the Hermes daemon",
		RunE: func(cmd *cobra.Command, args []string) error {
			return hermes.Run(hermes.RunOptions{
				ConfigPath: flagConfig,
				Port:       flagPort,
				WorkDir:    flagWorkDir,
				Provider:   flagProvider,
				Model:      flagModel,
				MultiAgent: flagMultiAgent,
				Sandbox:    flagSandbox,
				Daemon:     flagDaemon,
				Verbose:    flagVerbose,
				Debug:      flagDebug,
			}, version)
		},
	}

	startFlags := startCmd.Flags()
	startFlags.IntVar(&flagPort, "port", 0, "Listen port (default: from hermes.json or 8090)")
	startFlags.StringVar(&flagWorkDir, "work-dir", "", "Default working directory")
	startFlags.StringVar(&flagConfig, "config", "", "Path to hermes.json")
	startFlags.StringVarP(&flagProvider, "provider", "p", "", "Default provider name (overrides hermes.json)")
	startFlags.StringVarP(&flagModel, "model", "m", "", "Default model ID (overrides hermes.json)")
	startFlags.BoolVar(&flagMultiAgent, "multi-agent", false, "Enable multi-agent mode (sub-agent tools)")
	startFlags.BoolVar(&flagSandbox, "sandbox", false, "Enable sandbox mode (bwrap)")
	startFlags.BoolVarP(&flagDaemon, "daemon", "d", false, "Run in background")
	startFlags.BoolVar(&flagVerbose, "verbose", false, "Verbose output")
	startFlags.BoolVar(&flagDebug, "debug", false, "Enable debug logging")

	stopCmd := &cobra.Command{
		Use:   "stop",
		Short: "Stop the Hermes daemon",
		RunE: func(cmd *cobra.Command, args []string) error {
			pid, err := hermes.ReadPIDFile()
			if err != nil {
				return fmt.Errorf("read PID file: %w", err)
			}
			if pid == 0 {
				return fmt.Errorf("hermes is not running (no PID file found)")
			}
			proc, err := os.FindProcess(pid)
			if err != nil {
				return fmt.Errorf("find process %d: %w", pid, err)
			}
			if err := proc.Signal(syscall.SIGTERM); err != nil {
				return fmt.Errorf("send SIGTERM to process %d: %w", pid, err)
			}
			fmt.Fprintf(os.Stderr, "Sent SIGTERM to hermes (PID %d)\n", pid)
			return nil
		},
	}

	statusCmd := &cobra.Command{
		Use:   "status",
		Short: "Show Hermes daemon status",
		RunE: func(cmd *cobra.Command, args []string) error {
			pid, err := hermes.ReadPIDFile()
			if err != nil {
				return fmt.Errorf("read PID file: %w", err)
			}
			if pid == 0 {
				fmt.Fprintln(os.Stderr, "Hermes is not running (no PID file found)")
				return nil
			}
			// Check if process is alive
			proc, err := os.FindProcess(pid)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Hermes PID %d: process not found\n", pid)
				return nil
			}
			if err := proc.Signal(syscall.Signal(0)); err != nil {
				fmt.Fprintf(os.Stderr, "Hermes PID %d: not running\n", pid)
				return nil
			}
			fmt.Fprintf(os.Stderr, "Hermes is running (PID %d)\n", pid)

			// Try to query HTTP status
			cfg, err := hermes.LoadHermesConfig()
			if err == nil {
				url := fmt.Sprintf("http://%s/api/health", cfg.GetListenAddr())
				client := &http.Client{Timeout: 2 * time.Second}
				resp, err := client.Get(url)
				if err == nil {
					defer resp.Body.Close()
					var health map[string]any
					json.NewDecoder(resp.Body).Decode(&health)
					if v, ok := health["version"]; ok {
						fmt.Fprintf(os.Stderr, "  Version: %v\n", v)
					}
					if v, ok := health["uptime_seconds"]; ok {
						fmt.Fprintf(os.Stderr, "  Uptime: %v seconds\n", v)
					}
				}
			}
			return nil
		},
	}

	// --- config ---

	configCmd := &cobra.Command{
		Use:   "config",
		Short: "Manage Hermes configuration",
	}

	var flagProject, flagGlobal, flagWebhook bool

	configInitCmd := &cobra.Command{
		Use:   "init",
		Short: "Create hermes.json config template",
		RunE: func(cmd *cobra.Command, args []string) error {
			if flagProject && flagGlobal {
				return fmt.Errorf("--project and --global are mutually exclusive")
			}
			if flagWebhook {
				path, err := hermes.InitWebhookConfig(flagProject, flagForce)
				if err != nil {
					return err
				}
				fmt.Fprintf(os.Stderr, "Created webhook config: %s\n", path)
				fmt.Fprintf(os.Stderr, "\nSample routes:\n")
				fmt.Fprintf(os.Stderr, "  POST /webhook/github  — GitHub events (push, pull_request, issues)\n")
				fmt.Fprintf(os.Stderr, "  POST /webhook/ci      — CI events (all types)\n")
				fmt.Fprintf(os.Stderr, "\nSet WEBHOOK_SECRET env var or replace ${WEBHOOK_SECRET} in config.\n")
				return nil
			}
			path, err := hermes.InitHermesConfig(flagProject, flagForce)
			if err != nil {
				return err
			}
			fmt.Fprintf(os.Stderr, "Created hermes config: %s\n", path)
			return nil
		},
	}

	configInitCmd.Flags().BoolVar(&flagProject, "project", false, "Write to .vibe/hermes.json")
	configInitCmd.Flags().BoolVar(&flagGlobal, "global", false, "Write to global hermes.json (default)")
	configInitCmd.Flags().BoolVar(&flagForce, "force", false, "Overwrite existing file")
	configInitCmd.Flags().BoolVar(&flagWebhook, "webhook", false, "Include sample webhook routes (GitHub, CI)")

	configShowCmd := &cobra.Command{
		Use:   "show",
		Short: "Show current effective configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := hermes.LoadHermesConfig()
			if err != nil {
				return err
			}
			data, _ := json.MarshalIndent(cfg, "", "  ")
			fmt.Println(string(data))
			return nil
		},
	}

	configCmd.AddCommand(configInitCmd, configShowCmd)

	// --- client ---

	var flagURL, flagSession string

	clientCmd := &cobra.Command{
		Use:   "client",
		Short: "Connect to a running Hermes instance via WebSocket",
		RunE: func(cmd *cobra.Command, args []string) error {
			return hermes.RunClient(hermes.ClientOptions{
				URL:       flagURL,
				SessionID: flagSession,
			})
		},
	}
	clientCmd.Flags().StringVar(&flagURL, "url", "ws://localhost:8090/ws", "WebSocket URL to connect to")
	clientCmd.Flags().StringVar(&flagSession, "session", "", "Session ID to resume")

	// --- wechat ---

	wechatCmd := &cobra.Command{
		Use:   "wechat",
		Short: "Manage WeChat iLink connection",
	}

	wechatLoginCmd := &cobra.Command{
		Use:   "login",
		Short: "Login to WeChat via QR code",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := hermes.LoadHermesConfig()
			if err != nil {
				return err
			}
			credPath := cfg.GetWechatCredPath()
			client := wechat.NewClient()
			_, err = wechat.Login(cmd.Context(), client, wechat.LoginOptions{
				CredPath: credPath,
				Force:    flagForce,
			})
			if err != nil {
				return err
			}
			fmt.Fprintf(os.Stderr, "WeChat credentials saved to %s\n", credPath)
			return nil
		},
	}
	wechatLoginCmd.Flags().BoolVar(&flagForce, "force", false, "Force re-login even if credentials exist")

	wechatStatusCmd := &cobra.Command{
		Use:   "status",
		Short: "Show WeChat connection status",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := hermes.LoadHermesConfig()
			if err != nil {
				return err
			}
			credPath := cfg.GetWechatCredPath()
			creds, err := wechat.LoadCredentials(credPath)
			if err != nil || creds == nil {
				fmt.Fprintln(os.Stderr, "WeChat: not logged in")
				fmt.Fprintf(os.Stderr, "  Run: vibecoding hermes wechat login\n")
				return nil
			}
			fmt.Fprintf(os.Stderr, "WeChat: logged in\n")
			fmt.Fprintf(os.Stderr, "  UserID: %s\n", creds.UserID)
			fmt.Fprintf(os.Stderr, "  AccountID: %s\n", creds.AccountID)
			fmt.Fprintf(os.Stderr, "  SavedAt: %s\n", creds.SavedAt)
			fmt.Fprintf(os.Stderr, "  CredPath: %s\n", credPath)
			return nil
		},
	}

	wechatCmd.AddCommand(wechatLoginCmd, wechatStatusCmd)

	// --- feishu ---

	feishuCmd := &cobra.Command{
		Use:   "feishu",
		Short: "Manage Feishu (Lark) connection",
	}

	feishuSetupCmd := &cobra.Command{
		Use:   "setup",
		Short: "Configure Feishu app credentials",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(os.Stderr, "Configure Feishu app credentials in hermes.json:")
			fmt.Fprintln(os.Stderr, "")
			fmt.Fprintln(os.Stderr, `  "feishu": {`)
			fmt.Fprintln(os.Stderr, `    "enabled": true,`)
			fmt.Fprintln(os.Stderr, `    "app_id": "cli_xxxx",`)
			fmt.Fprintln(os.Stderr, `    "app_secret": "xxxx"`)
			fmt.Fprintln(os.Stderr, `  }`)
			fmt.Fprintln(os.Stderr, "")
			fmt.Fprintln(os.Stderr, "Or set environment variables: FEISHU_APP_ID, FEISHU_APP_SECRET")
			fmt.Fprintln(os.Stderr, "")
			fmt.Fprintln(os.Stderr, "Steps:")
			fmt.Fprintln(os.Stderr, "  1. Go to https://open.feishu.cn → Create App")
			fmt.Fprintln(os.Stderr, "  2. Enable Bot capability")
			fmt.Fprintln(os.Stderr, "  3. Subscribe to im.message.receive_v1 event")
			fmt.Fprintln(os.Stderr, "  4. Copy App ID and App Secret to hermes.json")
			return nil
		},
	}

	feishuStatusCmd := &cobra.Command{
		Use:   "status",
		Short: "Show Feishu connection status",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := hermes.LoadHermesConfig()
			if err != nil {
				return err
			}
			if !cfg.Feishu.Enabled {
				fmt.Fprintln(os.Stderr, "Feishu: disabled")
				return nil
			}
			if cfg.Feishu.AppID == "" || cfg.Feishu.AppSecret == "" {
				fmt.Fprintln(os.Stderr, "Feishu: enabled but not configured")
				fmt.Fprintln(os.Stderr, "  Run: vibecoding hermes feishu setup")
				return nil
			}
			fmt.Fprintln(os.Stderr, "Feishu: configured")
			fmt.Fprintf(os.Stderr, "  AppID: %s\n", cfg.Feishu.AppID)
			fmt.Fprintf(os.Stderr, "  WorkDir: %s\n", cfg.GetPlatformWorkDir("feishu"))
			return nil
		},
	}

	feishuCmd.AddCommand(feishuSetupCmd, feishuStatusCmd)

	// --- cron ---

	cronCmd := newCronCommand()

	// --- assemble ---

	hermesCmd.AddCommand(startCmd, stopCmd, statusCmd, configCmd, clientCmd, wechatCmd, feishuCmd, cronCmd)

	// --- webhook ---

	webhookCmd := &cobra.Command{
		Use:   "webhook",
		Short: "Manage webhook routes",
	}

	webhookListCmd := &cobra.Command{
		Use:   "list",
		Short: "List configured webhook routes",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := hermes.LoadHermesConfig()
			if err != nil {
				return err
			}
			if !cfg.Webhooks.Enabled {
				fmt.Println("Webhooks: disabled")
				return nil
			}
			if len(cfg.Webhooks.Routes) == 0 {
				fmt.Println("No webhook routes configured.")
				return nil
			}
			fmt.Printf("Webhooks: enabled (secret: %v)\n", cfg.Webhooks.Secret != "")
			for _, r := range cfg.Webhooks.Routes {
				events := "*"
				if len(r.Events) > 0 {
					events = fmt.Sprintf("%v", r.Events)
				}
				fmt.Printf("  POST /webhook%s  events=%s  skill=%s  delivery=%s\n", r.Path, events, r.Skill, r.Delivery)
			}
			return nil
		},
	}

	webhookCmd.AddCommand(webhookListCmd)

	// --- memory ---

	memoryCmd := &cobra.Command{
		Use:   "memory",
		Short: "Manage persistent memory",
	}

	memoryShowCmd := &cobra.Command{
		Use:   "show",
		Short: "Show current memory.md content",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := hermes.LoadHermesConfig()
			if err != nil {
				return err
			}
			cfg.GetWorkDir() // ensure work dir resolved
			store := memory.NewStore(cfg.Memory.Path, cfg.GetWorkDir())
			content, path, source, err := store.Read()
			if err != nil {
				return err
			}
			if content == "" {
				fmt.Println("No memory file found.")
				return nil
			}
			fmt.Fprintf(os.Stderr, "Source: %s — %s\n\n", source, path)
			fmt.Println(content)
			return nil
		},
	}

	memoryClearCmd := &cobra.Command{
		Use:   "clear",
		Short: "Clear memory.md content",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := hermes.LoadHermesConfig()
			if err != nil {
				return err
			}
			store := memory.NewStore(cfg.Memory.Path, cfg.GetWorkDir())
			if err := store.WriteAll("# Agent Memory\n\n## User Profile\n\n## Working Memory\n\n## Lessons Learned\n"); err != nil {
				return err
			}
			fmt.Println("Memory cleared.")
			return nil
		},
	}

	memoryCmd.AddCommand(memoryShowCmd, memoryClearCmd)

	// --- sessions ---

	sessionsCmd := &cobra.Command{
		Use:   "sessions",
		Short: "Manage hermes sessions",
	}

	sessionsListCmd := &cobra.Command{
		Use:   "list",
		Short: "List active sessions (queries running instance)",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := hermes.LoadHermesConfig()
			if err != nil {
				return err
			}
			url := fmt.Sprintf("http://%s/api/sessions", cfg.GetListenAddr())
			client := &http.Client{Timeout: 2 * time.Second}
			resp, err := client.Get(url)
			if err != nil {
				return fmt.Errorf("cannot reach hermes: %w (is it running?)", err)
			}
			defer resp.Body.Close()
			var result map[string]any
			json.NewDecoder(resp.Body).Decode(&result)
			data, _ := json.MarshalIndent(result, "", "  ")
			fmt.Println(string(data))
			return nil
		},
	}

	sessionsCmd.AddCommand(sessionsListCmd)

	hermesCmd.AddCommand(webhookCmd, memoryCmd, sessionsCmd)

	return hermesCmd
}

// newCronCommand builds the "cron" subcommand tree.
func newCronCommand() *cobra.Command {
	var (
		flagSchedule  string
		flagOneShot   bool
		flagA2ATarget string
		flagA2AToken  string
	)

	cronCmd := &cobra.Command{
		Use:   "cron",
		Short: "Manage cron scheduled tasks",
	}

	cronListCmd := &cobra.Command{
		Use:   "list",
		Short: "List all cron jobs",
		RunE: func(cmd *cobra.Command, args []string) error {
			store := openCronStore()
			jobs, err := store.List()
			if err != nil {
				return err
			}
			if len(jobs) == 0 {
				fmt.Println("No cron jobs.")
				return nil
			}
			for _, j := range jobs {
				enabled := "✅"
				if !j.Enabled {
					enabled = "⏸"
				}
				kind := "periodic"
				if j.OneShot {
					kind = "one-shot"
				}
				fmt.Printf("%s [%s] %s (%s, %s, runs: %d)\n", enabled, j.ID, j.Name, kind, j.Schedule, j.RunCount)
			}
			return nil
		},
	}

	cronAddCmd := &cobra.Command{
		Use:   "add <name> <prompt>",
		Short: "Add a cron job",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			store := openCronStore()
			name := args[0]
			prompt := args[1]
			job, err := store.Create(cron.CronJob{
				Name:      name,
				Prompt:    prompt,
				Schedule:  flagSchedule,
				OneShot:   flagOneShot,
				Enabled:   true,
				Mode:      "yolo",
				A2ATarget: flagA2ATarget,
				A2AToken:  flagA2AToken,
			})
			if err != nil {
				return err
			}
			fmt.Printf("✅ Created: [%s] %s\n", job.ID, job.Name)
			if job.A2ATarget != "" {
				fmt.Printf("   A2A Target: %s\n", job.A2ATarget)
			}
			return nil
		},
	}
	cronAddCmd.Flags().StringVar(&flagSchedule, "schedule", "", "Schedule: @daily, @weekly, @every 30m, etc.")
	cronAddCmd.Flags().BoolVar(&flagOneShot, "oneshot", false, "One-shot task (auto-disable after first run)")
	cronAddCmd.Flags().StringVar(&flagA2ATarget, "a2a-target", "", "A2A server URL (send task via A2A protocol)")
	cronAddCmd.Flags().StringVar(&flagA2AToken, "a2a-token", "", "Bearer token for A2A server")

	cronRemoveCmd := &cobra.Command{
		Use:   "remove <id>",
		Short: "Remove a cron job",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store := openCronStore()
			if err := store.Delete(args[0]); err != nil {
				return err
			}
			fmt.Printf("🗑 Removed: %s\n", args[0])
			return nil
		},
	}

	cronEnableCmd := &cobra.Command{
		Use:   "enable <id>",
		Short: "Enable a cron job",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return setCronEnabled(args[0], true)
		},
	}

	cronDisableCmd := &cobra.Command{
		Use:   "disable <id>",
		Short: "Disable a cron job",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return setCronEnabled(args[0], false)
		},
	}

	cronCmd.AddCommand(cronListCmd, cronAddCmd, cronRemoveCmd, cronEnableCmd, cronDisableCmd)
	return cronCmd
}
