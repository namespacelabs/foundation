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
           .=#@@@%*+=-::..::-=*#@@@%+:
         :#@@@+:                 :=%@@%=
       :#@@*-                       .+@@@-
      *@@#.                            +@@%.
     #@@+                               :%@@:
    #@@:      =%%#.     -%%*             .%@@.
   *@@-       +@@@@=    =@@#               %@%
  :@@#        +@@@@@#   =@@#               -@@=
  +@@-        +@@*=@@%: =@@#                @@@
  #@@.        +@@* .#@@+=@@#                %@@
  *@@.        +@@*   +@@@@@#                %@@
  +@@=        +@@*    :@@@@#                @@%
  .@@%        +@@*      +@@#               =@@-
   =@@=                                   .@@#
    *@@=                 #%%%%%%%%%+     :@@%.
     *@@*                *#########=    -@@%.
      =@@%-                           :*@@*
        +@@%=                       -#@@#:
         .+%@@#=:               .=*@@@*:
            :+%@@@%*+=-----=+*#@@@@#-
               .:=*#%@@@@@@@%#*+-.
`

func WelcomeMessage(firstRun bool) string {
	var b strings.Builder

	b.WriteString(logo)
	b.WriteString("\n\nThank you for trying Namespace!\n\n")

	if firstRun {
		// Add this to make clear that this large help won't be printed on every command.
		b.WriteString("It appears this is your first run of `ns`. Let us provice some help on how to interact with it:\n")
	}

	// TODO add more content.
	b.WriteString("We have assembled some canonical examples at namespacelabs.dev/examples.\n")
	b.WriteString("Our full documentation is located at docs.namespace.so.\n")

	if firstRun {
		b.WriteString("To see this message again at a later point, just run `ns --help`.\n")
	}

	return b.String()
}

func PrintWelcome(ctx context.Context, firstRun bool) {
	fmt.Fprint(console.TypedOutput(ctx, "welcome", console.CatOutputUs), WelcomeMessage(firstRun))
}
