package vagrantexec

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/dominodatalab/vagrant-exec/command"
	log "github.com/sirupsen/logrus"
)

const binary = "vagrant"

// Vagrant defines the interface for executing Vagrant commands
type Vagrant interface {
	Up() error
	Halt() error
	Destroy() error
	Status() ([]MachineStatus, error)
	Version() (string, error)
	SSH(string) (string, error)

	PluginList() ([]Plugin, error)
	PluginInstall(Plugin) error
}

// Plugin encapsulates Vagrant plugin metadata.
type Plugin struct {
	Name     string
	Version  string
	Location string
}

// wrapper is the default implementation of the Vagrant Interface.
type wrapper struct {
	executable string
	runner     command.Runner
	logger     log.FieldLogger
}

// New creates a new Vagrant CLI wrapper.
func New() Vagrant {
	logger := log.New()

	return wrapper{
		executable: binary,
		logger:     logger,
		runner:     command.ShellRunner{},
	}
}

// Up creates and configures guest machines according to your Vagrantfile.
func (w wrapper) Up() error {
	out, err := w.exec("up")
	if err == nil {
		w.info(out)
	}
	return err
}

// Halt will gracefully shut down the guest operating system and power down the guest machine.
func (w wrapper) Halt() error {
	out, err := w.exec("halt")
	if err == nil {
		w.info(out)
	}
	return err
}

// Destroy stops the running guest machines and destroys all of the resources created during the creation process.
func (w wrapper) Destroy() error {
	out, err := w.exec("destroy", "--force")
	if err == nil {
		w.info(out)
	}
	return err
}

// Status reports the status of the machines Vagrant is managing.
func (w wrapper) Status() (statuses []MachineStatus, err error) {
	out, err := w.exec("status", "--machine-readable")
	if err != nil {
		return
	}
	machineInfo, err := parseMachineReadable(out)
	if err != nil {
		return
	}

	statusMap := map[string]*MachineStatus{}
	for _, entry := range machineInfo {
		if len(entry.target) == 0 {
			continue // skip when no target specified
		}

		var status *MachineStatus // fetch status or create when missing
		status, ok := statusMap[entry.target]
		if !ok {
			status = &MachineStatus{Name: entry.target}
			statusMap[entry.target] = status
		}

		switch entry.mType { // populate status fields
		case "provider-name":
			status.Provider = entry.data[0]
		case "state":
			status.State = ToMachineState(entry.data[0])
		}
	}

	for _, st := range statusMap {
		statuses = append(statuses, *st)
	}
	return statuses, nil
}

// Version displays the current version of Vagrant you have installed.
func (w wrapper) Version() (version string, err error) {
	out, err := w.exec("version", "--machine-readable")
	if err != nil {
		return
	}
	vInfo, err := parseMachineReadable(out)
	if err != nil {
		return
	}
	data, err := pluckEntryData(vInfo, "version-installed")
	if err != nil {
		return
	}

	return data[0], err
}

// SSH executes a command on a Vagrant machine via SSH and returns the stdout/stderr output.
func (w wrapper) SSH(command string) (string, error) {
	out, err := w.exec("ssh", "--no-tty", "--command", command)
	return string(out), err
}

// PluginList returns a list of all installed plugins, their versions and install locations.
func (w wrapper) PluginList() (plugins []Plugin, err error) {
	out, err := w.exec("plugin", "list", "--machine-readable")
	if err != nil {
		return
	}
	pluginInfo, err := parseMachineReadable(out)
	if err != nil {
		return
	}
	pluginMetadataExtractor := regexp.MustCompile(`^([\w-]+)\s\((.*)%!\(VAGRANT_COMMA\)\s([a-z]+)\)$`)
	for _, entry := range pluginInfo {
		if entry.mType == "ui" {
			matches := pluginMetadataExtractor.FindAllStringSubmatch(entry.data[1], -1)[0][1:]
			plugins = append(plugins, Plugin{
				Name:     matches[0],
				Version:  matches[1],
				Location: matches[2],
			})
		}
	}
	return
}

// PluginInstall installs a plugin with the given name or file path.
func (w wrapper) PluginInstall(plugin Plugin) error {
	if len(plugin.Name) == 0 {
		return errors.New("plugin must have a name")
	}
	cmdArgs := []string{"plugin", "install", plugin.Name}

	if len(plugin.Version) > 0 {
		cmdArgs = append(cmdArgs, "--plugin-version", plugin.Version)
	}
	if plugin.Location == "local" {
		cmdArgs = append(cmdArgs, "--local")
	}

	out, err := w.exec(cmdArgs...)
	if err == nil {
		w.info(out)
	}
	return err
}

// exec dispatches vagrant commands via the shell runner.
func (w wrapper) exec(args ...string) ([]byte, error) {
	fullCmd := fmt.Sprintf("%s %s", w.executable, strings.Join(args, " "))

	w.logger.Infof("Running command [%s]", fullCmd)
	bs, err := w.runner.Execute(w.executable, args...)
	w.logger.Debugf("Command output [%s]: %s", fullCmd, bs)

	return bs, err
}

// info will log non-empty input.
func (w wrapper) info(out []byte) {
	if len(out) > 0 {
		w.logger.Info(string(out))
	}
}
