package colors

import (
	"hash/fnv"

	"github.com/fatih/color"
)

func DeterministicColorFunc(id string) func(format string, a ...interface{}) string {
	options := []func(format string, a ...interface{}) string{
		color.CyanString,
		color.GreenString,
		color.BlueString,
		color.MagentaString,
		color.HiBlueString,
		color.HiCyanString,
		color.HiGreenString,
		color.HiMagentaString,
	}

	h := fnv.New32a()
	h.Write([]byte(id))
	hashedValue := h.Sum32()

	return options[hashedValue%uint32(len(options))]
}
