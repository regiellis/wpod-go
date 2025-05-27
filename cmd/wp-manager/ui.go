/*
 * wpod - WordPress management tool
 * Copyright (C) 2025 Regi E
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <https://www.gnu.org/licenses/>.
 */

package main

import (
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

// --- Lipgloss Styles and Huh Theme ---
var (
	colorPrimary      = lipgloss.Color("#FACC15")
	colorSecondary    = lipgloss.Color("#1542FA")
	colorSuccess      = lipgloss.Color("#BAFA15")
	colorError        = lipgloss.Color("#FA5E15")
	colorWarning      = lipgloss.Color("#A58403")
	colorInfo         = lipgloss.Color("#15FACB")
	colorMuted        = lipgloss.Color("#D9CD9B")
	colorTextInput    = lipgloss.Color("#FEFEFF")
	colorTextEmphasis = lipgloss.Color("#FFFFFF")

	appTitleStyle      = lipgloss.NewStyle().Bold(true).Foreground(colorPrimary).PaddingBottom(1)
	sectionHeaderStyle = lipgloss.NewStyle().Bold(true).Foreground(colorSecondary).MarginTop(1).MarginBottom(1)
	successMsgStyle    = lipgloss.NewStyle().MarginBottom(1).Foreground(colorSuccess)
	errorMsgStyle      = lipgloss.NewStyle().MarginBottom(1).Foreground(colorError)
	warningMsgStyle    = lipgloss.NewStyle().MarginBottom(1).Foreground(colorWarning)
	infoMsgStyle       = lipgloss.NewStyle().MarginBottom(1).Foreground(colorInfo)
	commandStyle       = lipgloss.NewStyle().Foreground(colorPrimary).Bold(true)
	boldStyle          = lipgloss.NewStyle().Bold(true)
	subtleStyle        = lipgloss.NewStyle().Foreground(colorMuted)
	warningTitle       = lipgloss.NewStyle().Bold(true).Foreground(colorWarning)
	infoBox            = lipgloss.NewStyle().Padding(0, 1).Border(lipgloss.RoundedBorder(), false, false, false, true).BorderForeground(colorMuted)

	tableHeaderStyle   = lipgloss.NewStyle().Bold(true).Foreground(colorTextEmphasis).Padding(0, 1)
	tableCellStyle     = lipgloss.NewStyle().Padding(0, 1).Foreground(colorInfo)
	tableBorderStyle   = lipgloss.NewStyle().Border(lipgloss.HiddenBorder())
	statusRunningStyle = lipgloss.NewStyle().Foreground(colorSuccess)
	statusStoppedStyle = lipgloss.NewStyle().Foreground(colorWarning)
	statusErrorStyle   = lipgloss.NewStyle().Foreground(colorError)

	theme *huh.Theme
)

var (
	useLightTheme bool
	lightFlagSet  bool
)
