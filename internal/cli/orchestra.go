package cli

import (
	"github.com/spf13/cobra"
)

// newOrchestraCmd creates the orchestra root command.
// @AX:ANCHOR: [AUTO] CLI entry point — registers all 7 orchestra subcommands; changes here affect all orchestra routes
func newOrchestraCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "orchestra",
		Short: "다중 모델 오케스트레이션으로 코드를 분석한다",
		Long: `orchestra는 여러 코딩 CLI를 동시에 실행하여 합의, 파이프라인,
토론, 최속 전략으로 결과를 병합하는 다중 모델 오케스트레이션 엔진입니다.`,
	}

	cmd.AddCommand(newOrchestraReviewCmd())
	cmd.AddCommand(newOrchestraPlanCmd())
	cmd.AddCommand(newOrchestraSecureCmd())
	cmd.AddCommand(newOrchestraBrainstormCmd())
	cmd.AddCommand(newOrchestraJobStatusCmd())
	cmd.AddCommand(newOrchestraJobWaitCmd())
	cmd.AddCommand(newOrchestraJobResultCmd())
	cmd.AddCommand(newOrchestraCollectCmd())
	cmd.AddCommand(newOrchestraCleanupCmd())
	cmd.AddCommand(newOrchestraInjectCmd())
	cmd.AddCommand(newOrchestraRunCmd())

	return cmd
}

// newOrchestraReviewCmd creates the code review subcommand.
func newOrchestraReviewCmd() *cobra.Command {
	var (
		strategy  string
		providers []string
		timeout   int
		judge     string
		rounds    int
		noDetach  bool
		noJudge   bool
	)

	cmd := &cobra.Command{
		Use:   "review [files...]",
		Short: "여러 모델로 코드를 리뷰한다",
		Long:  "여러 코딩 CLI를 사용하여 지정된 파일을 리뷰하고 결과를 병합합니다.",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Pass only explicitly set flags; empty string means "use config"
			flagStrategy := flagStringIfChanged(cmd, "strategy", strategy)
			flagProviders := flagStringSliceIfChanged(cmd, "providers", providers)
			keepRelay, _ := cmd.Flags().GetBool("keep-relay-output")
			thresholdFlag, _ := cmd.Flags().GetFloat64("threshold")
			resolvedRounds := resolveRounds(flagStrategy, rounds)
			prompt := buildReviewPrompt(args)
			return runOrchestraCommand(cmd.Context(), "review", flagStrategy, flagProviders, timeout, judge, prompt, resolvedRounds, thresholdFlag, OrchestraFlags{NoDetach: noDetach, KeepRelay: keepRelay, NoJudge: noJudge})
		},
	}

	cmd.Flags().StringVarP(&strategy, "strategy", "s", "", "오케스트레이션 전략 (consensus|pipeline|debate|fastest|relay)")
	cmd.Flags().StringSliceVarP(&providers, "providers", "p", nil, "사용할 프로바이더 목록")
	cmd.Flags().IntVarP(&timeout, "timeout", "t", 120, "타임아웃 (초)")
	cmd.Flags().StringVar(&judge, "judge", "", "debate 전략에서 최종 판정 프로바이더")
	cmd.Flags().Float64("threshold", 0, "consensus 전략 합의 임계값 (0.0-1.0)")
	cmd.Flags().IntVar(&rounds, "rounds", 0, "debate 라운드 수 (1-10, debate 전략 전용)")
	cmd.Flags().BoolVar(&noDetach, "no-detach", false, "Disable auto-detach mode")
	cmd.Flags().Bool("keep-relay-output", false, "relay 전략 실행 후 임시 파일 보존")
	cmd.Flags().BoolVar(&noJudge, "no-judge", false, "Skip judge verdict phase in debate strategy")

	return cmd
}

// newOrchestraPlanCmd creates the plan subcommand.
func newOrchestraPlanCmd() *cobra.Command {
	var (
		strategy  string
		providers []string
		timeout   int
		rounds    int
		noDetach  bool
	)

	cmd := &cobra.Command{
		Use:   "plan \"description\"",
		Short: "여러 모델로 구현 계획을 수립한다",
		Long:  "여러 코딩 CLI를 사용하여 기능 구현 계획을 합의 방식으로 수립합니다.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			flagStrategy := flagStringIfChanged(cmd, "strategy", strategy)
			flagProviders := flagStringSliceIfChanged(cmd, "providers", providers)
			keepRelay, _ := cmd.Flags().GetBool("keep-relay-output")
			thresholdFlag, _ := cmd.Flags().GetFloat64("threshold")
			resolvedRounds := resolveRounds(flagStrategy, rounds)
			prompt := args[0]
			return runOrchestraCommand(cmd.Context(), "plan", flagStrategy, flagProviders, timeout, "", prompt, resolvedRounds, thresholdFlag, OrchestraFlags{NoDetach: noDetach, KeepRelay: keepRelay})
		},
	}

	cmd.Flags().StringVarP(&strategy, "strategy", "s", "", "오케스트레이션 전략 (consensus|pipeline|debate|fastest|relay)")
	cmd.Flags().StringSliceVarP(&providers, "providers", "p", nil, "사용할 프로바이더 목록")
	cmd.Flags().IntVarP(&timeout, "timeout", "t", 120, "타임아웃 (초)")
	cmd.Flags().Float64("threshold", 0, "consensus 전략 합의 임계값 (0.0-1.0)")
	cmd.Flags().IntVar(&rounds, "rounds", 0, "debate 라운드 수 (1-10, debate 전략 전용)")
	cmd.Flags().BoolVar(&noDetach, "no-detach", false, "Disable auto-detach mode")
	cmd.Flags().Bool("keep-relay-output", false, "relay 전략 실행 후 임시 파일 보존")

	return cmd
}

// newOrchestraSecureCmd creates the security analysis subcommand.
func newOrchestraSecureCmd() *cobra.Command {
	var (
		strategy  string
		providers []string
		timeout   int
		rounds    int
		noDetach  bool
	)

	cmd := &cobra.Command{
		Use:   "secure [files...]",
		Short: "여러 모델로 보안 취약점을 분석한다",
		Long:  "여러 코딩 CLI를 사용하여 지정된 파일의 보안 취약점을 분석합니다.",
		RunE: func(cmd *cobra.Command, args []string) error {
			flagStrategy := flagStringIfChanged(cmd, "strategy", strategy)
			flagProviders := flagStringSliceIfChanged(cmd, "providers", providers)
			keepRelay, _ := cmd.Flags().GetBool("keep-relay-output")
			thresholdFlag, _ := cmd.Flags().GetFloat64("threshold")
			resolvedRounds := resolveRounds(flagStrategy, rounds)
			prompt := buildSecurePrompt(args)
			return runOrchestraCommand(cmd.Context(), "secure", flagStrategy, flagProviders, timeout, "", prompt, resolvedRounds, thresholdFlag, OrchestraFlags{NoDetach: noDetach, KeepRelay: keepRelay})
		},
	}

	cmd.Flags().StringVarP(&strategy, "strategy", "s", "", "오케스트레이션 전략 (consensus|pipeline|debate|fastest|relay)")
	cmd.Flags().StringSliceVarP(&providers, "providers", "p", nil, "사용할 프로바이더 목록")
	cmd.Flags().IntVarP(&timeout, "timeout", "t", 120, "타임아웃 (초)")
	cmd.Flags().Float64("threshold", 0, "consensus 전략 합의 임계값 (0.0-1.0)")
	cmd.Flags().IntVar(&rounds, "rounds", 0, "debate 라운드 수 (1-10, debate 전략 전용)")
	cmd.Flags().BoolVar(&noDetach, "no-detach", false, "Disable auto-detach mode")
	cmd.Flags().Bool("keep-relay-output", false, "relay 전략 실행 후 임시 파일 보존")

	return cmd
}
