package container

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/jedib0t/go-pretty/v6/table"
	c "github.com/scotty-c/prox/pkg/client"
	"github.com/scotty-c/prox/pkg/output"
)

// AvailableTemplate is a template that can be downloaded with 'ct templates pull'
type AvailableTemplate struct {
	Name        string `json:"name"` // file name
	OS          string `json:"os"`
	Version     string `json:"version"`
	Description string `json:"description,omitempty"`
}

// toAvailable converts aplinfo entries, keeping only LXC system templates
// and parsing OS and version the same way as for downloaded templates
func toAvailable(apps []c.Appliance) []AvailableTemplate {
	var out []AvailableTemplate
	for _, a := range apps {
		if a.Template == "" || (a.Section != "" && a.Section != "system") {
			continue
		}
		out = append(out, AvailableTemplate{
			Name:        a.Template,
			OS:          parseTemplateOS(a.Template),
			Version:     parseTemplateVersion(a.Template),
			Description: a.Description,
		})
	}
	return out
}

// matchAvailable finds the template for want, which is a file name or an
// os:version shortcut like those accepted by 'ct create'
func matchAvailable(templates []AvailableTemplate, want string) (AvailableTemplate, error) {
	for _, t := range templates {
		if t.Name == want {
			return t, nil
		}
	}

	osName, version, ok := strings.Cut(want, ":")
	if !ok {
		return AvailableTemplate{}, fmt.Errorf("template %q not found; use a name from 'prox ct templates --available' or os:version, e.g. debian:12", want)
	}
	var matches []AvailableTemplate
	for _, t := range templates {
		if strings.EqualFold(t.OS, osName) && t.Version == version {
			matches = append(matches, t)
		}
	}
	if len(matches) == 0 {
		return AvailableTemplate{}, fmt.Errorf("no downloadable template for %s; see 'prox ct templates --available'", want)
	}
	// Prefer the "standard" flavour, then the newest (names sort by build)
	slices.SortFunc(matches, func(a, b AvailableTemplate) int { return strings.Compare(b.Name, a.Name) })
	for _, m := range matches {
		if strings.Contains(m.Name, "-standard_") {
			return m, nil
		}
	}
	return matches[0], nil
}

// pickTemplateStorage chooses where to download a template: "local" if it
// can hold templates, otherwise the first storage that can
func pickTemplateStorage(available []string) (string, error) {
	if len(available) == 0 {
		return "", fmt.Errorf("no storage on this node can hold container templates; use --storage")
	}
	if slices.Contains(available, "local") {
		return "local", nil
	}
	return available[0], nil
}

// firstOnlineNode returns node if given, otherwise the first online node
func firstOnlineNode(ctx context.Context, client c.ProxmoxClientInterface, node string) (string, error) {
	if node != "" {
		return node, nil
	}
	resources, err := client.GetClusterResources(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get cluster resources: %w", err)
	}
	for _, r := range resources {
		if r.Type == "node" && r.Status == "online" {
			return r.Node, nil
		}
	}
	return "", fmt.Errorf("no online nodes found")
}

// ListAvailableTemplates prints the templates node (or the first online
// node) can download
func ListAvailableTemplates(node string) error {
	ctx := context.Background()
	client, err := c.CreateClient()
	if err != nil {
		output.ClientError(err)
		return fmt.Errorf("failed to create client: %w", err)
	}
	if node, err = firstOnlineNode(ctx, client, node); err != nil {
		return err
	}

	apps, err := client.GetAvailableTemplates(ctx, node)
	if err != nil {
		return fmt.Errorf("failed to list downloadable templates: %w", err)
	}
	templates := toAvailable(apps)
	slices.SortFunc(templates, func(a, b AvailableTemplate) int { return strings.Compare(a.Name, b.Name) })

	if output.Structured() {
		if templates == nil {
			templates = []AvailableTemplate{}
		}
		return output.OutputJSON(templates)
	}

	t := table.NewWriter()
	t.SetTitle(fmt.Sprintf("Downloadable Templates (node %s)", node))
	t.AppendHeader(table.Row{"OS", "VERSION", "TEMPLATE"})
	for _, tmpl := range templates {
		t.AppendRow(table.Row{tmpl.OS, tmpl.Version, tmpl.Name})
	}
	t.SetStyle(table.StyleRounded)
	output.Result("\n%s\n", t.Render())
	output.Result("Found %d template(s). Download one with: prox ct templates pull <os:version>\n", len(templates))
	return nil
}

// PullTemplate downloads a template to node (or the first online node).
// want is a file name or an os:version shortcut. storage "" picks one.
func PullTemplate(want, node, storage string, wait bool) error {
	ctx := context.Background()
	client, err := c.CreateClient()
	if err != nil {
		output.ClientError(err)
		return fmt.Errorf("failed to create client: %w", err)
	}
	if node, err = firstOnlineNode(ctx, client, node); err != nil {
		return err
	}

	apps, err := client.GetAvailableTemplates(ctx, node)
	if err != nil {
		return fmt.Errorf("failed to list downloadable templates: %w", err)
	}
	tmpl, err := matchAvailable(toAvailable(apps), want)
	if err != nil {
		return err
	}

	if storage == "" {
		names, err := client.GetStorageNames(ctx, node, "vztmpl")
		if err != nil {
			return fmt.Errorf("failed to list storage: %w", err)
		}
		if storage, err = pickTemplateStorage(names); err != nil {
			return err
		}
	}

	// Skip the download if it's already there
	if existing, err := client.GetContainerTemplates(ctx, node); err == nil {
		for _, e := range existing {
			if e.VolID == storage+":vztmpl/"+tmpl.Name {
				output.Result("Template %s is already on %s (node %s)\n", tmpl.Name, storage, node)
				return nil
			}
		}
	}

	output.Info("Downloading %s to %s on node %s...\n", tmpl.Name, storage, node)
	taskID, err := client.DownloadTemplate(ctx, node, storage, tmpl.Name)
	if err != nil {
		return fmt.Errorf("failed to start download: %w", err)
	}
	return finishTask(ctx, client, node, taskID, wait, fmt.Sprintf("Downloaded %s to %s", tmpl.Name, storage))
}
