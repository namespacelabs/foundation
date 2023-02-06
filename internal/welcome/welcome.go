// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package welcome

import (
	"context"
	"fmt"
	"strings"

	"namespacelabs.dev/foundation/internal/console"
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
	var b strings.Builder

	b.WriteString(logo)

	product := "Namespace"
	if cmd == "nsc" {
		product = "Namespace Cloud"
	}

	b.WriteString(fmt.Sprintf("\n\nThank you for trying %s!\n\n", product))

	if firstRun {
		// Add this to make clear that this large help won't be printed on every command.
		b.WriteString(fmt.Sprintf("It appears this is your first run of `%s`. Let us provide some help on how to interact with it:\n", cmd))
	}

	switch cmd {
	case "ns":
		// TODO add more content.
		b.WriteString("We have assembled some canonical examples at https://namespacelabs.dev/examples.\n")
		b.WriteString("Our full documentation is located at https://namespace.so/docs.\n")

	case "nsc":
		b.WriteString("Get started at https://cloud.namespace.so/.\n")
		// TODO
	}

	b.WriteString("Tell us what you think on https://community.namespace.so/discord/.\n")

	if firstRun {
		b.WriteString(fmt.Sprintf("To see this message again at a later point, just run `%s --help`.\n", cmd))
	}

	return b.String()
}

func PrintWelcome(ctx context.Context, firstRun bool, cmd string) {
	fmt.Fprint(console.TypedOutput(ctx, "welcome", console.CatOutputUs), WelcomeMessage(firstRun, cmd))
}
