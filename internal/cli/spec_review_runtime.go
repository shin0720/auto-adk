package cli

import (
	"github.com/shin0720/auto-adk/pkg/orchestra"
	"github.com/shin0720/auto-adk/pkg/spec"
)

var (
	specReviewRunOrchestra   = orchestra.RunOrchestra
	specReviewBuildProviders = buildReviewProviders
)

func syncReviewedSpecStatus(specDir string, result *spec.ReviewResult) error {
	if result == nil {
		return nil
	}
	if result.Verdict != spec.VerdictPass || hasActiveFindings(result.Findings) {
		return nil
	}
	return spec.UpdateStatus(specDir, "approved")
}
