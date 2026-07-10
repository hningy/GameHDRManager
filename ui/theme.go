package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

var (
	bg      = color.NRGBA{R: 13, G: 17, B: 23, A: 255}
	panel   = color.NRGBA{R: 22, G: 27, B: 34, A: 255}
	card    = color.NRGBA{R: 33, G: 38, B: 45, A: 255}
	border  = color.NRGBA{R: 48, G: 54, B: 61, A: 255}
	text    = color.NRGBA{R: 230, G: 237, B: 243, A: 255}
	textDim = color.NRGBA{R: 139, G: 148, B: 158, A: 255}
	accent  = color.NRGBA{R: 88, G: 166, B: 255, A: 255}
	success = color.NRGBA{R: 63, G: 185, B: 80, A: 255}
	warning = color.NRGBA{R: 210, G: 153, B: 34, A: 255}
	danger  = color.NRGBA{R: 248, G: 81, B: 73, A: 255}
)

type darkTheme struct{}

func (darkTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	switch name {
	case theme.ColorNameBackground:
		return bg
	case theme.ColorNameOverlayBackground, theme.ColorNameMenuBackground, theme.ColorNameHeaderBackground:
		return panel
	case theme.ColorNameButton:
		return card
	case theme.ColorNameDisabledButton:
		return border
	case theme.ColorNamePrimary:
		return accent
	case theme.ColorNameForeground:
		return text
	case theme.ColorNameInputBackground:
		return bg
	case theme.ColorNameInputBorder, theme.ColorNameSeparator:
		return border
	case theme.ColorNameHover, theme.ColorNamePressed, theme.ColorNameSelection, theme.ColorNameFocus:
		return border
	case theme.ColorNamePlaceHolder:
		return textDim
	case theme.ColorNameDisabled:
		return textDim
	case theme.ColorNameError:
		return danger
	case theme.ColorNameSuccess:
		return success
	}
	return theme.DefaultTheme().Color(name, variant)
}
func (darkTheme) Font(style fyne.TextStyle) fyne.Resource    { return theme.DefaultTheme().Font(style) }
func (darkTheme) Icon(name fyne.ThemeIconName) fyne.Resource { return theme.DefaultTheme().Icon(name) }
func (darkTheme) Size(name fyne.ThemeSizeName) float32       { return theme.DefaultTheme().Size(name) }
