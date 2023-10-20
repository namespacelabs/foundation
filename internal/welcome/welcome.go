// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package welcome

import (
	"fmt"
	"strings"
)

const logo = `
                     .:::::..
               :=*%@@@@@@@@@@@%#+-
           .=#@@@%*+=-::''::-=*#@@@%+:
         :#@@@+:                 :=%@@%=
       :#@@*-                       '+@@@-
      *@@#'                            +@@%.
     #@@+                               :%@@:
    #@@:      =%%#.     -%%*             '%@@.
   *@@-       +@@@@=    =@@#               %@%
  :@@#        +@@@@@#   =@@#               -@@=
  +@@=        +@@*=@@%: =@@#                @@@
  #@@|        +@@* '#@@+=@@#                %@@
  *@@|        +@@*   +@@@@@#                %@@
  +@@=        +@@*    :@@@@#                @@%
  '@@%        +@@*      +@@#               =@@-
   =@@=                                   .@@#
    *@@=                 #%%%%%%%%%+     :@@%'
     *@@*                *#########=    -@@%'
      =@@%-                           :*@@*
        +@@%=                       -#@@#:
         '+%@@#=:               .=*@@@*:
            :+%@@@%*+=-----=+*#@@@@#-
               ':=*#%@@@@@@@%#*+-'
`

func WelcomeMessage(firstRun bool, cmd string) string {
	if cmd == "nsc" {
		return `Thank you for trying Namespace Cloud!

Get started at https://cloud.namespace.so/
`
	}

	var b strings.Builder

	b.WriteString(logo)
	b.WriteString("\n\nThank you for trying Namespace!\n\n")

	if firstRun {
		// Add this to make clear that this large help won't be printed on every command.
		b.WriteString(fmt.Sprintf("It appears this is your first run of `%s`. Let us provide some help on how to interact with it:\n", cmd))
	}

	b.WriteString(`We have assembled some canonical examples at https://namespacelabs.dev/examples.
Our full documentation is located at https://namespace.so/docs.
Tell us what you think on https://community.namespace.so/discord/.
`)

	if firstRun {
		b.WriteString(fmt.Sprintf("To see this message again at a later point, just run `%s --help`.\n", cmd))
	}

	return b.String()
}
