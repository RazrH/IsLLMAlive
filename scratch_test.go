package main

import (
	"encoding/json"
	"fmt"
	"os"
)

type spSummary struct {
	Status     spStatus      `json:"status"`
	Components []spComponent `json:"components"`
}

type spStatus struct {
	Indicator   string `json:"indicator"`
	Description string `json:"description"`
}

type spComponent struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

func main() {
	for _, filename := range []string{"example1.txt", "example2.txt", "exampleOAI.txt"} {
		data, err := os.ReadFile("d:/Gemini/IsLLMAlive/plans/" + filename)
		if err != nil {
			fmt.Printf("%s: read err: %v\n", filename, err)
			continue
		}
		var summary spSummary
		err = json.Unmarshal(data, &summary)
		if err != nil {
			fmt.Printf("%s: parse err: %v\n", filename, err)
		} else {
			fmt.Printf("%s: parsed OK. Indicator: %s, Components: %d\n", filename, summary.Status.Indicator, len(summary.Components))
		}
	}
}
