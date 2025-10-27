package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/costinm/grpc-mesh/pkg/echo"
	"github.com/costinm/meshauth"
	k8sc "github.com/costinm/mk8s"
	"github.com/costinm/mk8s/gcp"
	"github.com/costinm/ugate/appinit"
	"github.com/costinm/ugate/cmd"

	"github.com/fsnotify/fsnotify"
	"github.com/mitchellh/mapstructure"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// TODO: use reflection and appinit to generate the commands.
// Eventually multiple registries should be used, and maybe some
// scripting used to parse and make MCP or http requests.

var rootCmd = &cobra.Command{
	Run: func(corbaCMD *cobra.Command, args []string) {

		// Override std flag: flag.Lookup("n").Value.Set("new")

		// This gets called if there are no commands (only flags)
		// Args follow "--"
		log.Println("Run", args, corbaCMD.Flags().Lookup("n"), corbaCMD.Flags())
		corbaCMD.Help()

		// VisitAll - including defaults.
		// Visit - doesn't seem to work with cobra.
		flag.Visit(func(f *flag.Flag) {
			log.Println("Stdflag", f.Name, f.Value)
		})

		// flag.CommandLine is the static default.
		// flag set can be used to create separate sets of flgs that can be parsed
		// conditionally.

		fs := flag.NewFlagSet("test", flag.ContinueOnError)
		fs.Bool("test", false, "test")

	},
}

// Execute is the entry point for Cobra main.
// TODO: generated code, using comments in the pkg/apis
func CobraExecute() {
	//rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.cobra.yaml)")

	rootCmd.PersistentFlags().AddGoFlagSet(flag.CommandLine)

	AddCobra(rootCmd)

	debug := false
	rootCmd.PersistentFlags().BoolVar(&debug, "debug", false, "Debug")

	// Viper is used to load config from file, env, flags.
	viper.SetConfigName("ugate")
	viper.AddConfigPath(".")
	viper.AddConfigPath(os.Getenv("HOME") + "/.ssh")

	viper.AutomaticEnv()
	viper.BindPFlags(pflag.CommandLine)
	//viper.BindFlagValues(flag.CommandLine)

	// Bind will set flags from the config file.
	// Not clear why - instead of the opposite.
	viper.BindPFlag("author", rootCmd.PersistentFlags().Lookup("author"))
	viper.BindPFlag("projectbase", rootCmd.PersistentFlags().Lookup("projectbase"))
	viper.BindPFlag("useViper", rootCmd.PersistentFlags().Lookup("viper"))

	viper.SetDefault("n", "istio-system")

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// Nothing
		} else {
			log.Println("Config file error:", err)
		}
	}

	viper.OnConfigChange(func(e fsnotify.Event) {
		fmt.Println("Config file changed:", e.Name)
	})
	//go viper.WatchConfig()
	rootCmd.AddCommand(&cobra.Command{
		Use: "viper",
		Run: func(cmd *cobra.Command, args []string) {
			a := viper.AllSettings()
			ab, _ := json.MarshalIndent(a, "", "  ")
			fmt.Println(string(ab))
		},
	})

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

// AddCobra commands to the root
func AddCobra(root *cobra.Command) {
	ctx := context.Background()

	for k, v := range appinit.AppCodec().Range() {
		fmt.Println("Registering", k, v)
	}

	AddEchoCommand(root)

	AddTokenCommand(root)

	AddJWTCommand(root)

	AddCSRSignCommand(root, ctx)

	//AddCertReqCommand(root)
	AddSDCommand(root)

	sdcca := &cobra.Command{
		Use: "q query ollama",
		Run: func(c *cobra.Command, args []string) {
			cmd.RegisterOllama()
		},
	}
	root.AddCommand(sdcca)

}

func AddSDCommand(root *cobra.Command) {
	obj := &gcp.MetricListRequest{}
	sdcca := &cobra.Command{
		Use: "sda handles stackdriver commands",
		Run: func(cmd *cobra.Command, args []string) {
			m := map[string]interface{}{}
			cmd.Flags().Visit(func(f *pflag.Flag) {
				m[f.Name] = f.Value.String()
			})
			mapstructure.Decode(m, obj)

			obj.Run(cmd.Context())
		},
	}
	ParseStruct(sdcca, obj)
	root.AddCommand(sdcca)

	sdcc := &cobra.Command{
		Use: "sd handles stackdriver commands",
	}
	sdcc.PersistentFlags().StringVar(&obj.ProjectID, "project", "", "Project ID")
	sdcc.PersistentFlags().StringVar(&obj.Name, "name", "", "Name - metric:[ID], log:[ID]. If ID is missing, will list logs and metrics")
	sdcc.PersistentFlags().StringVar(&obj.Name, "filter", "", "Filter")

	subc := &cobra.Command{
		Use: "l handles google stackdriver logs",
		Run: func(cc *cobra.Command, args []string) {
			ctx := context.Background()
			if len(args) > 0 {
				obj.Name = args[0]
			}

			err := gcp.LogList(ctx, obj)
			if err != nil {
				log.Fatal("Error", err)
			}

		}}
	subc.PersistentFlags().BoolVar(&obj.Watch, "watch", false, "Watch the logs")
	sdcc.AddCommand(subc)

	subc = &cobra.Command{
		Use: "m lists stackdriver metrics",
		Run: func(cc *cobra.Command, args []string) {
			ctx := context.Background()
			if len(args) > 0 {
				obj.Name = args[0]
			}

			err := gcp.MetricList(ctx, obj)

			if err != nil {
				log.Fatal("Error", err)
			}

		}}
	subc.PersistentFlags().BoolVar(&obj.Active, "active", false, "Slow: only list active metrics")
	sdcc.AddCommand(subc)

	subc = &cobra.Command{
		Use: "mset sets a metric value",
		Run: func(cc *cobra.Command, args []string) {
			ctx := context.Background()
			gcp.MetricUpdate(ctx, obj)
		}}
	subc.PersistentFlags().Float64Var(&obj.Val, "v", 0, "Metric value")
	sdcc.AddCommand(subc)

	subc = &cobra.Command{
		Use: "res list resources",
		Run: func(cc *cobra.Command, args []string) {
			ctx := context.Background()
			obj.ListResources(ctx)
		}}
	subc.PersistentFlags().Float64Var(&obj.Val, "v", 0, "Metric value")
	sdcc.AddCommand(subc)

}

func AddCSRSignCommand(root *cobra.Command, ctx context.Context) {
	root.AddCommand(&cobra.Command{
		Use:  "csrsignd",
		Long: "Runs the CSR signer",
		Run: func(cc *cobra.Command, args []string) {

			k := k8sc.Default()
			if k.Default == nil {
				log.Fatal("No cluster")
			}

			cmd.CSRSignD(ctx, k)
			select {}
		},
	})
}

func AddEchoCommand(root *cobra.Command) {
	secho := &echo.EchoClientReq{}
	cmdecho := &cobra.Command{
		Use: "echo",

		Run: func(cmd *cobra.Command, args []string) {
			ctx := context.Background()
			echo.Client(ctx, secho)
		}}
	cmdecho.PersistentFlags().StringVar(&secho.Addr, "addr", "", "Address in grpc format")
	root.AddCommand(cmdecho)
}

func AddJWTCommand(root *cobra.Command) {
	cmtok := &cobra.Command{
		Use: "jwtdecode",

		Run: func(cobraCMD *cobra.Command, args []string) {

			data, err := io.ReadAll(os.Stdin)
			if err != nil {
				log.Fatal(err)
			}
			tok := meshauth.DecodeJWT(string(data))

			fmt.Println(tok.String())
		}}
	rootCmd.AddCommand(cmtok)
}

func AddTokenCommand(root *cobra.Command) {
	tr := &cmd.TokenRequest{}
	cmtok := &cobra.Command{
		Use: "token",

		Run: func(cobraCMD *cobra.Command, args []string) {
			ctx := context.Background()
			res, err := tr.Run(ctx)

			if err != nil {
				log.Fatal(err)
			}
			fmt.Println(res)
		}}
	cmtok.Flags().StringVar(&tr.Aud, "aud", "", "Audience. IF empty, return access token for GSA, K8S APIserver audience for K8S")
	cmtok.PersistentFlags().StringVar(&tr.KSA, "ksa", "default", "Get token for a K8S service account")
	cmtok.PersistentFlags().StringVar(&tr.Namespace, "n", "default", "Get a token for KSA in namespace")
	cmtok.PersistentFlags().StringVar(&tr.GCPSA, "gsa", "", "Get token for a Google service account")
	cmtok.PersistentFlags().BoolVar(&tr.Fed, "fed", false, "Get a google federated token. Aud is ignored.")
	root.AddCommand(cmtok)
}
