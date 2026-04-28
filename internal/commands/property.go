package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strconv"

	"github.com/urfave/cli/v3"

	"github.com/major/jira-agent/internal/client"
	apperr "github.com/major/jira-agent/internal/errors"
	"github.com/major/jira-agent/internal/output"
)

type propertyAPI struct {
	get    func(context.Context, string, map[string]string, any) error
	put    func(context.Context, string, any, any) error
	delete func(context.Context, string, any) error
}

type propertyTarget struct {
	resourceName string
	idLabel      string
	basePath     func(string) (string, string, error)
	api          propertyAPI
}

func issuePropertyCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cli.Command {
	return propertyCommand(issuePropertyTarget(apiClient), w, format, allowWrites)
}

func issuePropertyTarget(apiClient *client.Ref) propertyTarget {
	return propertyTarget{
		resourceName: "issue",
		idLabel:      "issue key",
		basePath: func(issueKey string) (string, string, error) {
			if issueKey == "" {
				return "", "", apperr.NewValidationError("issue key is required", nil)
			}
			return "/issue/" + escapePathSegment(issueKey) + "/properties", issueKey, nil
		},
		api: propertyAPI{
			get: func(ctx context.Context, path string, params map[string]string, result any) error {
				return apiClient.Get(ctx, path, params, result)
			},
			put: func(ctx context.Context, path string, body, result any) error {
				return apiClient.Put(ctx, path, body, result)
			},
			delete: func(ctx context.Context, path string, result any) error {
				return apiClient.Delete(ctx, path, result)
			},
		},
	}
}

func projectPropertyCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cli.Command {
	return propertyCommand(projectPropertyTarget(apiClient), w, format, allowWrites)
}

func projectPropertyTarget(apiClient *client.Ref) propertyTarget {
	return propertyTarget{
		resourceName: "project",
		idLabel:      "project key",
		basePath: func(projectKey string) (string, string, error) {
			if projectKey == "" {
				return "", "", apperr.NewValidationError("project key is required", nil)
			}
			return "/project/" + escapePathSegment(projectKey) + "/properties", projectKey, nil
		},
		api: propertyAPI{
			get: func(ctx context.Context, path string, params map[string]string, result any) error {
				return apiClient.Get(ctx, path, params, result)
			},
			put: func(ctx context.Context, path string, body, result any) error {
				return apiClient.Put(ctx, path, body, result)
			},
			delete: func(ctx context.Context, path string, result any) error {
				return apiClient.Delete(ctx, path, result)
			},
		},
	}
}

func sprintPropertyCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cli.Command {
	return propertyCommand(sprintPropertyTarget(apiClient), w, format, allowWrites)
}

func sprintPropertyTarget(apiClient *client.Ref) propertyTarget {
	return propertyTarget{
		resourceName: "sprint",
		idLabel:      "sprint ID",
		basePath: func(sprintID string) (string, string, error) {
			id, err := parseSprintID(sprintID)
			if err != nil {
				return "", "", err
			}
			canonicalID := strconv.FormatInt(id, 10)
			return "/sprint/" + canonicalID + "/properties", canonicalID, nil
		},
		api: propertyAPI{
			get: func(ctx context.Context, path string, params map[string]string, result any) error {
				return apiClient.AgileGet(ctx, path, params, result)
			},
			put: func(ctx context.Context, path string, body, result any) error {
				return apiClient.AgilePut(ctx, path, body, result)
			},
			delete: func(ctx context.Context, path string, result any) error {
				return apiClient.AgileDelete(ctx, path, result)
			},
		},
	}
}

func boardPropertyCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cli.Command {
	return propertyCommand(boardPropertyTarget(apiClient), w, format, allowWrites)
}

func boardPropertyTarget(apiClient *client.Ref) propertyTarget {
	return propertyTarget{
		resourceName: "board",
		idLabel:      "board ID",
		basePath: func(boardID string) (string, string, error) {
			id, err := parseBoardID(boardID)
			if err != nil {
				return "", "", err
			}
			canonicalID := strconv.FormatInt(id, 10)
			return "/board/" + canonicalID + "/properties", canonicalID, nil
		},
		api: propertyAPI{
			get: func(ctx context.Context, path string, params map[string]string, result any) error {
				return apiClient.AgileGet(ctx, path, params, result)
			},
			put: func(ctx context.Context, path string, body, result any) error {
				return apiClient.AgilePut(ctx, path, body, result)
			},
			delete: func(ctx context.Context, path string, result any) error {
				return apiClient.AgileDelete(ctx, path, result)
			},
		},
	}
}

func propertyCommand(target propertyTarget, w io.Writer, format *output.Format, allowWrites *bool) *cli.Command {
	return &cli.Command{
		Name:  "property",
		Usage: fmt.Sprintf("Manage %s properties", target.resourceName),
		UsageText: fmt.Sprintf(`jira-agent %s property list %s
jira-agent %s property get %s com.example.flag
jira-agent %s property set %s com.example.flag --value-json '{"enabled":true}'`,
			target.resourceName,
			exampleResourceID(target.resourceName),
			target.resourceName,
			exampleResourceID(target.resourceName),
			target.resourceName,
			exampleResourceID(target.resourceName),
		),
		DefaultCommand: "list",
		Commands: []*cli.Command{
			propertyListCommand(target, w, format),
			propertyGetCommand(target, w, format),
			propertySetCommand(target, w, format, allowWrites),
			propertyDeleteCommand(target, w, format, allowWrites),
		},
	}
}

func propertyListCommand(target propertyTarget, w io.Writer, format *output.Format) *cli.Command {
	return &cli.Command{
		Name:      "list",
		Usage:     fmt.Sprintf("List %s property keys", target.resourceName),
		UsageText: fmt.Sprintf(`jira-agent %s property list %s`, target.resourceName, exampleResourceID(target.resourceName)),
		ArgsUsage: "<" + target.idLabel + ">",
		Action: func(ctx context.Context, cmd *cli.Command) error {
			basePath, _, err := propertyBasePath(target, cmd)
			if err != nil {
				return err
			}
			return writeAPIResult(w, *format, func(result any) error {
				return target.api.get(ctx, basePath, nil, result)
			})
		},
	}
}

func propertyGetCommand(target propertyTarget, w io.Writer, format *output.Format) *cli.Command {
	return &cli.Command{
		Name:      "get",
		Usage:     fmt.Sprintf("Get a %s property", target.resourceName),
		UsageText: fmt.Sprintf(`jira-agent %s property get %s com.example.flag`, target.resourceName, exampleResourceID(target.resourceName)),
		ArgsUsage: "<" + target.idLabel + "> <property-key>",
		Action: func(ctx context.Context, cmd *cli.Command) error {
			basePath, propertyKey, err := propertyPathParts(target, cmd)
			if err != nil {
				return err
			}
			path := basePath + "/" + escapePathSegment(propertyKey)
			return writeAPIResult(w, *format, func(result any) error {
				return target.api.get(ctx, path, nil, result)
			})
		},
	}
}

func propertySetCommand(target propertyTarget, w io.Writer, format *output.Format, allowWrites *bool) *cli.Command {
	return &cli.Command{
		Name:      "set",
		Usage:     fmt.Sprintf("Set a %s property", target.resourceName),
		UsageText: fmt.Sprintf(`jira-agent %s property set %s com.example.flag --value-json '{"enabled":true}'`, target.resourceName, exampleResourceID(target.resourceName)),
		ArgsUsage: "<" + target.idLabel + "> <property-key>",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "value-json",
				Usage:    "Property value as raw JSON (required)",
				Required: true,
			},
		},
		Metadata: writeCommandMetadata(),
		Action: writeGuard(allowWrites, func(ctx context.Context, cmd *cli.Command) error {
			basePath, propertyKey, err := propertyPathParts(target, cmd)
			if err != nil {
				return err
			}
			value, err := parsePropertyValue(cmd.String("value-json"))
			if err != nil {
				return err
			}
			path := basePath + "/" + escapePathSegment(propertyKey)
			return writeAPIResult(w, *format, func(result any) error {
				return target.api.put(ctx, path, value, result)
			})
		}),
	}
}

func propertyDeleteCommand(target propertyTarget, w io.Writer, format *output.Format, allowWrites *bool) *cli.Command {
	return &cli.Command{
		Name:      "delete",
		Usage:     fmt.Sprintf("Delete a %s property", target.resourceName),
		UsageText: fmt.Sprintf(`jira-agent %s property delete %s com.example.flag`, target.resourceName, exampleResourceID(target.resourceName)),
		ArgsUsage: "<" + target.idLabel + "> <property-key>",
		Metadata:  writeCommandMetadata(),
		Action: writeGuard(allowWrites, func(ctx context.Context, cmd *cli.Command) error {
			basePath, propertyKey, err := propertyPathParts(target, cmd)
			if err != nil {
				return err
			}
			path := basePath + "/" + escapePathSegment(propertyKey)
			if err := target.api.delete(ctx, path, nil); err != nil {
				return err
			}
			return output.WriteResult(w, map[string]any{
				"resource":    target.resourceName,
				"propertyKey": propertyKey,
				"deleted":     true,
			}, *format)
		}),
	}
}

func propertyBasePath(target propertyTarget, cmd *cli.Command) (basePath, canonicalID string, err error) {
	resourceID, err := requireArg(cmd, target.idLabel)
	if err != nil {
		return "", "", err
	}
	return target.basePath(resourceID)
}

func propertyPathParts(target propertyTarget, cmd *cli.Command) (basePath, propertyKey string, err error) {
	args, err := requireArgs(cmd, target.idLabel, "property key")
	if err != nil {
		return "", "", err
	}
	basePath, _, err = target.basePath(args[0])
	if err != nil {
		return "", "", err
	}
	return basePath, args[1], nil
}

func parsePropertyValue(valueJSON string) (any, error) {
	if valueJSON == "" {
		return nil, apperr.NewValidationError("--value-json is required", nil)
	}

	var value any
	if err := json.Unmarshal([]byte(valueJSON), &value); err != nil {
		return nil, apperr.NewValidationError("--value-json must be valid JSON", err)
	}
	if value == nil {
		return nil, apperr.NewValidationError("--value-json must not be null", nil)
	}
	return value, nil
}

func exampleResourceID(resourceName string) string {
	switch resourceName {
	case "issue":
		return "PROJ-123"
	case "project":
		return "PROJ"
	case "sprint":
		return "100"
	case "board":
		return "42"
	default:
		return "ID"
	}
}
