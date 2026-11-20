package launcher

import (
	"fmt"
	"io"
	"path/filepath"

	"guard/internal/core"
	"guard/internal/hub/session"
	"guard/internal/i18n"
)

// BannerInfo holds runtime context displayed in the launcher briefing banner.
type BannerInfo struct {
	Mode           string
	SessionID      string
	Workspace      string
	PolicySource   string
	PolicyWarnings []string
	CmdName        string
	SkillPath      string
	SkillCreated   bool
	HookPath       string
	HookUpdated    bool
	HasHook        bool
}

// PrintBanner renders a concise visual briefing banner and quick usage hints before launching the child process.
func PrintBanner(w io.Writer, info BannerInfo) {
	if w == nil {
		return
	}
	upper := core.NormalizeMode(info.Mode)
	shortID := session.ShortID(info.SessionID)

	var title, blocked, allowed string
	switch upper {
	case core.ModeDev:
		title = i18n.T("launcher.banner.mode.dev.title")
		blocked = i18n.T("launcher.banner.mode.dev.blocked")
		allowed = i18n.T("launcher.banner.mode.dev.allowed")
	case core.ModeTest:
		title = i18n.T("launcher.banner.mode.test.title")
		blocked = i18n.T("launcher.banner.mode.test.blocked")
		allowed = i18n.T("launcher.banner.mode.test.allowed")
	case core.ModePlan:
		title = i18n.T("launcher.banner.mode.plan.title")
		blocked = i18n.T("launcher.banner.mode.plan.blocked")
		allowed = i18n.T("launcher.banner.mode.plan.allowed")
	case core.ModeAll, core.ModeQA:
		title = i18n.T("launcher.banner.mode.all.title", upper)
		blocked = i18n.T("launcher.banner.mode.all.blocked")
		allowed = i18n.T("launcher.banner.mode.all.allowed")
	default:
		title = upper
		blocked = i18n.T("launcher.banner.mode.default.blocked")
		allowed = i18n.T("launcher.banner.mode.default.allowed")
	}

	fmt.Fprint(w, i18n.T("launcher.banner.active_mode", title))
	fmt.Fprint(w, i18n.T("launcher.banner.session_id", shortID))
	fmt.Fprint(w, i18n.T("launcher.banner.workspace", info.Workspace))
	fmt.Fprint(w, i18n.T("launcher.banner.policy_source", info.PolicySource))
	for _, warn := range info.PolicyWarnings {
		fmt.Fprintf(w, "⚠️  [Policy Warning] %s\n", warn)
	}
	fmt.Fprint(w, i18n.T("launcher.banner.blocked", blocked))
	fmt.Fprint(w, i18n.T("launcher.banner.allowed", allowed))
	if info.SkillCreated && info.SkillPath != "" {
		fmt.Fprint(w, i18n.T("launcher.banner.skill_created", info.SkillPath))
	}
	if info.HookUpdated && info.HookPath != "" {
		fmt.Fprint(w, i18n.T("launcher.banner.hook_updated", info.HookPath))
	}
	if filepath.Base(info.CmdName) == "agy" && !info.HasHook {
		fmt.Fprint(w, i18n.T("launcher.banner.no_hook_warn"))
	}
	fmt.Fprint(w, i18n.T("launcher.banner.tip"))
	fmt.Fprint(w, i18n.T("launcher.banner.starting", info.CmdName))
}
