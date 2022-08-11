package api

import (
	"github.com/emirpasic/gods/sets/hashset"
	"github.com/rs/zerolog/log"
	"strings"
)

func shouldSkipCompletelyReplacedFlattenedList(psName string, lists map[string]interface{}, k string) bool {
	for eachName := range lists {
		if strings.HasPrefix(k, eachName+"[") {
			log.Info().Msgf("Skipping overridden list entry [%s] in source [%s]", k, psName)
			return true
		}
	}
	return false
}

func findCompletelyReplacedFlattenedLists(sources []PropertySource) []map[string]interface{} {
	listsToRemove := make([]map[string]interface{}, 0)
	for _, ps := range sources {
		listsToRemove = append(listsToRemove, findFlattenedLists(ps.Source))
	}

	//fmt.Println("Original", listsToRemove)

	listsSoFar := hashset.New()

	for i := len(sources) - 1; i >= 0; i-- { // reverse order
		for listName := range listsToRemove[i] {
			if !listsSoFar.Contains(listName) {
				// fmt.Println("First appearance of list - should be kept:", listName)
				listsSoFar.Add(listName)
				delete(listsToRemove[i], listName)
			}
		}
	}

	//fmt.Println("Filtered", listsToRemove)
	return listsToRemove
}

func findFlattenedLists(source map[string]interface{}) map[string]interface{} {
	var listNames = make(map[string]interface{})

	// Grab list names
	for propertyName, _ := range source {
		// Handle sublists recursively
		//switch valueType := value.(type) {
		//case map[string]interface{}:
		//	findFlattenedLists(valueType)
		//}

		if strings.HasSuffix(propertyName, "]") {
			idx := strings.IndexByte(propertyName, '[')
			if idx > 0 {
				listName := propertyName[:idx]
				listNames[listName] = true
			}
		}
	}

	return listNames
}
