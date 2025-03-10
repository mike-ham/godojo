package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"syscall"
	"time"
)

// OS commands to perform an action e.g. install DB from OS packages
type osCmds struct {
	id     string   // Holds distro + release e.g. ubuntu:18.04
	cmds   []string // Holds the os commands
	errmsg []string // Holds the error messages if the matching command fails
	hard   []bool   // Flag to know if an error on the matching command is fatal
}

func newCmds(d string) *osCmds {
	var o osCmds
	o.id = d
	return &o
}

func addCmd(o *osCmds, cmd string, lerr string, hard bool) {
	// Append command to existing list
	o.cmds = append(o.cmds, cmd)
	o.errmsg = append(o.cmds, lerr)
	o.hard = append(o.hard, hard)
}

// TODO: Document this and/or move it to a separate package
func sendCmd(o io.Writer, cmd string, lerr string, hard bool) {
	// Setup command
	runCmd := exec.Command("bash", "-c", cmd)
	_, err := o.Write([]byte("[godojo] # " + Redactatron(cmd, Redact) + "\n"))
	if err != nil {
		errorMsg(fmt.Sprintf("Failed to setup command, error was: %+v", err))
	}

	// Run and gather its output
	cmdOut, err := runCmd.CombinedOutput()
	if err != nil {
		errorMsg(fmt.Sprintf("%s - Failed to run OS command %+v, error was: %+v",
			timeStamp(), Redactatron(cmd, Redact), err))
		if hard {
			// Exit on hard aka fatal errors
			os.Exit(1)
		}
	}
	_, err = o.Write(cmdOut)
	if err != nil {
		errorMsg(fmt.Sprintf("Failed to write to OS command log file, error was: %+v", err))
	}
}

func runCmds(o io.Writer, c osCmds) {
	// Cycle through the provided commands, running them one at at time
	for i := range c.cmds {
		sendCmd(o,
			c.cmds[i],
			c.errmsg[i],
			c.hard[i])
	}
}

// TODO: Document this and/or move it to a separate package
func tryCmd(o io.Writer, cmd string, lerr string, hard bool) error {
	traceMsg("Entering tryCmd")
	// Setup command
	runCmd := exec.Command("bash", "-c", cmd)
	_, err := o.Write([]byte("[godojo] # " + Redactatron(cmd, Redact) + "\n"))
	if err != nil {
		traceMsg(fmt.Sprintf("Failed to setup command %+v, error was: %+v", Redactatron(cmd, Redact), err))
		return err
	}

	// Hook up stdout and strerr
	runCmd.Stdout = o
	runCmd.Stderr = o

	// Start the command
	err = runCmd.Start()
	if err != nil {
		traceMsg(fmt.Sprintf("Failed to start command, error was: %+v", err))
		return err
	}

	// Wait for command to exit, then check the exit code
	err = runCmd.Wait()
	if err != nil {
		// Check if the error is a ExitError
		if exiterr, ok := err.(*exec.ExitError); ok {
			// The program has exited with an exit code != 0
			if status, ok := exiterr.Sys().(syscall.WaitStatus); ok {
				// Above casts the exiterr to syscll.WaitStatus aka unint32
				traceMsg(fmt.Sprintf("%s - %s errored with exit status: %d", timeStamp(), cmd, status.ExitStatus()))
				return err
			}
		} else {
			traceMsg(fmt.Sprintf("%s - %s errored from Wait(): %v", timeStamp(), cmd, err))
			return err
		}
	}

	traceMsg("Non-error return from tryCmd")
	return nil
}

func tryCmds(o io.Writer, c osCmds) error {
	// Cycle through the provided commands, trying them one at at time
	for i := range c.cmds {
		err := tryCmd(o,
			c.cmds[i],
			c.errmsg[i],
			c.hard[i])

		if err != nil {
			traceMsg(fmt.Sprintf("%s - Command %s errored with %s. Underlying error is %+v",
				timeStamp(), c.cmds[i], c.errmsg[i], err))
			return errors.New(c.errmsg[i])
		}
	}

	return nil
}

// TODO: Document this and/or move it to a separate package
func inspectCmd(o io.Writer, cmd string, lerr string, hard bool) (string, error) {
	traceMsg("Inside inspectCmd")
	// Setup command
	runCmd := exec.Command("bash", "-c", cmd)
	_, err := o.Write([]byte("[godojo] # " + Redactatron(cmd, Redact) + "\n"))
	if err != nil {
		traceMsg(fmt.Sprintf("Failed to setup command, error was: %+v", err))
		return "", err
	}

	// Hook up stdout and strerr
	var tmpBuf bytes.Buffer
	multi := io.MultiWriter(o, &tmpBuf)
	if err != nil {
		traceMsg(fmt.Sprintf("Failed to setup MultiWriter, error was: %+v", err))
		return "", err
	}
	runCmd.Stdout = multi
	runCmd.Stderr = o

	// Start the command
	err = runCmd.Start()
	if err != nil {
		traceMsg(fmt.Sprintf("%s - Failed to start command %+v, error was: %+v", timeStamp(), Redactatron(cmd, Redact), err))
		return "", err
	}

	traceMsg("Before runCmd.Wait()")
	// Wait for command to exit, then check the exit code
	err = runCmd.Wait()
	if err != nil {
		// Check if the error is a ExitError
		if exiterr, ok := err.(*exec.ExitError); ok {
			// The program has exited with an exit code != 0
			if status, ok := exiterr.Sys().(syscall.WaitStatus); ok {
				// Above casts the exiterr to syscll.WaitStatus aka unint32
				traceMsg(fmt.Sprintf("%s - %s errored with exit status: %d", timeStamp(), cmd, status.ExitStatus()))
				return "", err
			}
		} else {
			traceMsg(fmt.Sprintf("%s - %s errored from Wait(): %v", timeStamp(), cmd, err))
			return "", err
		}
	}
	traceMsg("After runCmd.Wait()")

	traceMsg("Non-error return from inspectCmd")
	return tmpBuf.String(), nil
}

func inspectCmds(o io.Writer, c osCmds) ([]string, error) {
	traceMsg("Inside inspectCmds")
	ret := make([]string, 1)
	// Cycle through the provided commands, trying them one at at time
	for i := range c.cmds {
		traceMsg(fmt.Sprintf("Current cmd: %+v", c.cmds[i]))
		out, err := inspectCmd(o,
			c.cmds[i],
			c.errmsg[i],
			c.hard[i])

		if err != nil {
			traceMsg(fmt.Sprintf("%s - Command %s errored with %s. Underlying error is %+v",
				timeStamp(), c.cmds[i], c.errmsg[i], err))
			return ret, errors.New(c.errmsg[i])
		}
		ret = append(ret, out)
	}

	return ret, nil
}

func timeStamp() string {
	return time.Now().Format("2006/01/02 15:04:05")
}

// TODO: Write a version of impsectCmds that returns the exit code in addition to combined stderr & stdout to caller
