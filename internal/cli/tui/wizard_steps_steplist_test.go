package tui_test

import (
	"testing"

	"github.com/shin0720/auto-adk/internal/cli/tui"
	"github.com/stretchr/testify/assert"
)

// TestBuildStepList_StepCounts verifies step filtering based on flags.
func TestBuildStepList_StepCounts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		opts      tui.InitWizardOpts
		wantSteps int
	}{
		{
			name:      "all steps — no flags",
			opts:      tui.InitWizardOpts{},
			wantSteps: 5, // profile + lang + quality + review-gate + methodology
		},
		{
			name:      "quality pre-set — skip quality step",
			opts:      tui.InitWizardOpts{Quality: "ultra"},
			wantSteps: 4, // profile + lang + review-gate + methodology
		},
		{
			name:      "no-review-gate — skip gate step",
			opts:      tui.InitWizardOpts{NoReviewGate: true},
			wantSteps: 4, // profile + lang + quality + methodology
		},
		{
			name:      "both flags — skip quality and gate",
			opts:      tui.InitWizardOpts{Quality: "ultra", NoReviewGate: true},
			wantSteps: 3, // profile + lang + methodology
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			steps := tui.TestBuildStepList(tc.opts)
			assert.Len(t, steps, tc.wantSteps)
		})
	}
}

// TestBuildSteps_ReturnForms verifies each step builder produces a non-nil form.
func TestBuildSteps_ReturnForms(t *testing.T) {
	t.Parallel()

	result := &tui.InitWizardResult{}

	assert.NotNil(t, tui.TestBuildLangStep(1, 4, result))
	assert.NotNil(t, tui.TestBuildQualityStep(2, 4, result))
	assert.NotNil(t, tui.TestBuildMethodologyStep(4, 4, result))

	// Review gate with and without providers (covers both desc branches)
	assert.NotNil(t, tui.TestBuildReviewGateStep(3, 4, result,
		tui.InitWizardOpts{Providers: []string{"claude", "openai"}}))
	assert.NotNil(t, tui.TestBuildReviewGateStep(3, 4, result, tui.InitWizardOpts{}))
}

// TestBuildStepList_FormsCallable verifies all built steps produce runnable forms.
func TestBuildStepList_FormsCallable(t *testing.T) {
	t.Parallel()

	steps := tui.TestBuildStepList(tui.InitWizardOpts{})
	result := &tui.InitWizardResult{}
	for i, step := range steps {
		assert.NotNilf(t, step(result), "step %d should produce a non-nil form", i)
	}
}
