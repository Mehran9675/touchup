package components

import (
	"github.com/mehran9675/giode"
)

// actionState is the persistent part of an Action.
type actionState struct {
	b       *giode.ButtonElement
	onClick func()
}

// Action returns a button kept under (scope, key), built once by build,
// whose click handler is refreshed every frame. For buttons in lists.
// Buttons are kept per color mode, since build resolves theme colors.
func Action(scope string, key any, build func() *giode.ButtonElement, onClick func()) *giode.ButtonElement {
	st := giode.Keyed(scope+"/"+modeName(), key, func() *actionState {
		a := &actionState{b: build()}
		a.b.OnClick(func() {
			if a.onClick != nil {
				a.onClick()
			}
		})
		return a
	})
	st.onClick = onClick
	return st.b
}

func modeName() string {
	if giode.CurrentMode() == giode.ModeLight {
		return "light"
	}
	return "dark"
}
