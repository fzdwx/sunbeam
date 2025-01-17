package tui

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/pomdtr/sunbeam/app"
	"github.com/pomdtr/sunbeam/utils"
)

type CommandRunner struct {
	width, height int
	currentView   string

	extension NamedExtension
	command   NamedCommand

	with map[string]app.CommandInput

	header Header
	footer Footer

	list   *List
	detail *Detail
	form   *Form
}

type NamedExtension struct {
	app.Extension
	Name string
}

type NamedCommand struct {
	app.Command
	Name string
}

func (c CommandRunner) CheckEnv() error {
	requiredEnv := make([]string, 0, len(c.command.Env)+len(c.extension.Env))
	requiredEnv = append(requiredEnv, c.command.Env...)
	requiredEnv = append(requiredEnv, c.extension.Env...)

	for _, env := range requiredEnv {
		if _, ok := os.LookupEnv(env); !ok {
			return fmt.Errorf("missing env variable: %s", env)
		}
	}

	return nil
}

func NewCommandRunner(extension NamedExtension, command NamedCommand, with map[string]app.CommandInput) *CommandRunner {
	runner := CommandRunner{
		header:      NewHeader(),
		footer:      NewFooter(extension.Title),
		extension:   extension,
		currentView: "loading",
		command:     command,
	}

	// Copy the map to avoid modifying the original
	runner.with = make(map[string]app.CommandInput)
	for name, input := range with {
		runner.with[name] = input
	}

	return &runner
}
func (c *CommandRunner) Init() tea.Cmd {
	return tea.Sequence(c.SetIsloading(true), c.Run())
}

type CommandOutput []byte

func (c *CommandRunner) Run() tea.Cmd {
	formitems := make([]FormItem, 0)
	for _, param := range c.command.Params {
		input, ok := c.with[param.Name]
		if !ok {
			if param.Default != nil {
				continue
			} else {
				return NewErrorCmd(errors.New("missing required parameter: " + param.Name))
			}
		}

		if input.Value != nil {
			continue
		}

		formitems = append(formitems, NewFormItem(param.Name, input.FormItem))
	}

	// Show form if some parameters are set as input
	if len(formitems) > 0 {
		c.currentView = "form"
		c.form = NewForm(c.extension.Title, formitems)

		c.form.SetSize(c.width, c.height)
		return c.form.Init()
	}

	params := make(map[string]any)
	for name, input := range c.with {
		if input.Value == nil {
			continue
		}
		params[name] = input.Value
	}

	if err := c.CheckEnv(); err != nil {
		return NewErrorCmd(err)
	}

	if err := c.command.CheckMissingParams(params); err != nil {
		return NewErrorCmd(err)
	}

	commandInput := app.CommandParams{
		With: params,
	}

	if c.extension.Root.Scheme != "file" {
		if c.command.Interactive {
			return NewErrorCmd(fmt.Errorf("interactive commands are not supported for remote extensions"))
		}

		return c.RemoteRun(commandInput)
	}

	cmd, err := c.command.Cmd(commandInput, c.extension.Root.Path)
	if err != nil {
		return NewErrorCmd(err)
	}

	if c.command.Interactive {
		return tea.ExecProcess(cmd, func(err error) tea.Msg {
			if err != nil {
				return err
			}
			return CommandOutput([]byte{})
		})
	}

	return func() tea.Msg {
		output, err := cmd.Output()
		if err != nil {
			var exitErr *exec.ExitError
			if ok := errors.As(err, &exitErr); ok {
				return fmt.Errorf("command failed with exit code %d, error:\n%s", exitErr.ExitCode(), exitErr.Stderr)
			}
			return err
		}

		return CommandOutput(output)
	}
}

func (c CommandRunner) RemoteRun(commandParams app.CommandParams) tea.Cmd {
	return func() tea.Msg {
		payload, err := json.Marshal(commandParams)
		if err != nil {
			return err
		}

		commandUrl := url.URL{
			Scheme: c.extension.Root.Scheme,
			Host:   c.extension.Root.Host,
			Path:   path.Join(c.extension.Root.Path, c.command.Name),
		}
		res, err := http.Post(commandUrl.String(), "application/json", bytes.NewReader(payload))
		if err != nil {
			return err
		}
		defer res.Body.Close()

		if res.StatusCode != http.StatusOK {
			return fmt.Errorf("command failed with status code %d", res.StatusCode)
		}

		body, err := io.ReadAll(res.Body)
		if err != nil {
			return err
		}

		return CommandOutput(body)
	}
}

func (c *CommandRunner) SetIsloading(isLoading bool) tea.Cmd {
	switch c.currentView {
	case "list":
		return c.list.SetIsLoading(isLoading)
	case "detail":
		return c.detail.SetIsLoading(isLoading)
	case "form":
		return c.form.SetIsLoading(isLoading)
	case "loading":
		return c.header.SetIsLoading(isLoading)
	default:
		return nil
	}
}

func (c *CommandRunner) SetSize(width, height int) {
	c.width, c.height = width, height

	c.header.Width = width
	c.footer.Width = width

	switch c.currentView {
	case "list":
		c.list.SetSize(width, height)
	case "detail":
		c.detail.SetSize(width, height)
	case "form":
		c.form.SetSize(width, height)
	}
}

func (c *CommandRunner) Update(msg tea.Msg) (Page, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			if c.currentView != "loading" {
				break
			}
			return c, PopCmd
		}
	case CommandOutput:
		switch c.command.OnSuccess {
		case "push-page":
			var page app.Page
			var v any
			if err := json.Unmarshal(msg, &v); err != nil {
				return c, NewErrorCmd(err)
			}

			if err := app.PageSchema.Validate(v); err != nil {
				return c, NewErrorCmd(err)
			}

			err := json.Unmarshal(msg, &page)
			if err != nil {
				return c, NewErrorCmd(err)
			}

			if page.Title == "" {
				page.Title = c.extension.Title
			}

			switch page.Type {
			case "detail":
				c.currentView = "detail"
				c.detail = NewDetail(page.Title)

				actions := make([]Action, len(page.Detail.Actions))
				for i, scriptAction := range page.Detail.Actions {
					actions[i] = NewAction(scriptAction)
				}
				c.detail.SetActions(actions...)

				if page.Detail.Preview.Text != "" {
					c.detail.viewport.SetContent(page.Detail.Preview.Text)
				}

				if page.Detail.Preview.Command != "" {
					c.detail.PreviewCommand = func() string {
						command, ok := c.extension.Commands[page.Detail.Preview.Command]
						if !ok {
							return ""
						}

						params := app.CommandParams{
							With: page.Detail.Preview.With,
						}

						cmd, err := command.Cmd(params, c.extension.Root.Path)
						if err != nil {
							return err.Error()
						}

						output, err := cmd.Output()
						if err != nil {
							var exitErr *exec.ExitError
							if ok := errors.As(err, &exitErr); ok {
								return fmt.Sprintf("command failed with exit code %d, error:\n%s", exitErr.ExitCode(), exitErr.Stderr)
							}
							return err.Error()
						}

						return string(output)
					}
				}
				c.detail.SetSize(c.width, c.height)

				return c, c.detail.Init()
			case "list":
				c.currentView = "list"
				listItems := make([]ListItem, len(page.List.Items))
				for i, scriptItem := range page.List.Items {
					scriptItem := scriptItem

					if scriptItem.Id == "" {
						scriptItem.Id = strconv.Itoa(i)
					}

					listItem := ParseScriptItem(scriptItem)
					if scriptItem.Preview.Command != "" {
						listItem.PreviewCmd = func() string {
							command, ok := c.extension.Commands[scriptItem.Preview.Command]
							if !ok {
								return fmt.Sprintf("command %s not found", scriptItem.Preview.Command)
							}

							params := app.CommandParams{
								With: scriptItem.Preview.With,
							}
							if c.extension.Root.Scheme != "file" {
								payload, err := json.Marshal(params)
								if err != nil {
									return fmt.Sprintf("failed to marshal command params: %v", err)
								}

								commandUrl := url.URL{
									Scheme: c.extension.Root.Scheme,
									Host:   c.extension.Root.Host,
									Path:   path.Join(c.extension.Root.Path, scriptItem.Preview.Command),
								}
								res, err := http.Post(commandUrl.String(), "application/json", bytes.NewReader(payload))
								if err != nil {
									return fmt.Sprintf("failed to execute command: %v", err)
								}
								defer res.Body.Close()

								if res.StatusCode != http.StatusOK {
									return fmt.Sprintf("failed to execute command: %s", res.Status)
								}

								body, err := io.ReadAll(res.Body)
								if err != nil {
									return fmt.Sprintf("failed to read command output: %v", err)
								}

								return string(body)
							}

							cmd, err := command.Cmd(params, c.extension.Root.Path)
							if err != nil {
								return err.Error()
							}

							output, err := cmd.Output()
							if err != nil {
								var exitErr *exec.ExitError
								if ok := errors.As(err, &exitErr); ok {
									return fmt.Sprintf("command failed with exit code %d, error:\n%s", exitErr.ExitCode(), exitErr.Stderr)
								}
								return err.Error()
							}

							return string(output)
						}
					}

					listItems[i] = listItem
				}

				c.list = NewList(page.Title)
				c.list.filter.emptyText = page.List.EmptyText
				if page.List.ShowPreview {
					c.list.ShowPreview = true
				}

				c.list.SetItems(listItems)
				c.list.SetSize(c.width, c.height)

				return c, c.list.Init()
			}
		case "open-url":
			return c, NewOpenUrlCmd(string(msg))
		case "copy-text":
			return c, NewCopyTextCmd(string(msg))
		case "reload-page":
			return c, tea.Sequence(PopCmd, NewReloadPageCmd(nil))
		default:
			return c, tea.Quit
		}

	case SubmitFormMsg:
		for key, value := range msg.Values {
			c.with[key] = app.CommandInput{
				Value: value,
			}
		}

		c.currentView = "loading"

		return c, tea.Sequence(c.SetIsloading(true), c.Run())

	case RunCommandMsg:
		command, ok := c.extension.Commands[msg.Command]
		if !ok {
			return c, NewErrorCmd(fmt.Errorf("command not found: %s", msg.Command))
		}
		if msg.OnSuccess != "" {
			command.OnSuccess = msg.OnSuccess
		}

		c.Clear()

		return c, NewPushCmd(NewCommandRunner(c.extension, NamedCommand{
			Name:    msg.Command,
			Command: command,
		}, msg.With))

	case ReloadPageMsg:
		for key, value := range msg.With {
			c.with[key] = value
		}

		return c, tea.Sequence(c.SetIsloading(true), c.Run())
	}

	var cmd tea.Cmd
	var container Page

	switch c.currentView {
	case "list":
		container, cmd = c.list.Update(msg)
		c.list, _ = container.(*List)
	case "detail":
		container, cmd = c.detail.Update(msg)
		c.detail, _ = container.(*Detail)
	case "form":
		container, cmd = c.form.Update(msg)
		c.form, _ = container.(*Form)
	default:
		c.header, cmd = c.header.Update(msg)
	}
	return c, cmd
}

func (c *CommandRunner) Clear() {
	switch c.currentView {
	case "list":
		c.list.actions.Blur()
	case "detail":
		c.detail.actions.Blur()
	}
}

func (c *CommandRunner) View() string {
	switch c.currentView {
	case "list":
		return c.list.View()
	case "detail":
		return c.detail.View()
	case "form":
		return c.form.View()
	case "loading":
		headerView := c.header.View()
		footerView := c.footer.View()
		padding := make([]string, utils.Max(0, c.height-lipgloss.Height(headerView)-lipgloss.Height(footerView)))
		return lipgloss.JoinVertical(lipgloss.Left, c.header.View(), strings.Join(padding, "\n"), c.footer.View())
	default:
		return ""
	}
}
