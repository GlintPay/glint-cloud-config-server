package api

import (
	"github.com/GlintPay/gccs/utils"
	"strings"
)

type Sorter struct {
	AppNames []string
	Profiles []string
	Sources  []PropertySource
}

func (ps Sorter) Sort() func(i, j int) bool {
	return func(i, j int) bool {
		left := ps.Sources[i]
		adjustedLeftName := utils.StripGitPrefix(left.Name)
		// application.* is always bottom of the heap
		if strings.HasPrefix(adjustedLeftName, utils.BaseLevel) {
			return true
		}

		right := ps.Sources[j]
		adjustedRightName := utils.StripGitPrefix(right.Name)

		if strings.HasPrefix(adjustedRightName, utils.BaseLevel) {
			return false
		}

		// If both are application-env, find them among the requested profiles
		if strings.HasPrefix(adjustedLeftName, utils.DefaultApplicationNamePrefix) {
			if strings.HasPrefix(adjustedRightName, utils.DefaultApplicationNamePrefix) {
				leftProfileIdx := profileIndex(ps.Profiles, adjustedLeftName)
				rightProfileIdx := profileIndex(ps.Profiles, adjustedRightName)
				return leftProfileIdx > rightProfileIdx
			} else {
				return true
			}
		}

		// Now to resolve the different app / profile combinations
		// (1) Earlier app name is preferred

		leftNameIdx := appIndex(ps.AppNames, adjustedLeftName)
		rightNameIdx := appIndex(ps.AppNames, adjustedRightName)

		if leftNameIdx != rightNameIdx {
			return leftNameIdx > rightNameIdx // later is worse
		}

		// (2) Earlier matching profile name is preferred

		leftProfileIdx := profileIndex(ps.Profiles, adjustedLeftName)
		rightProfileIdx := profileIndex(ps.Profiles, adjustedRightName)

		return leftProfileIdx > rightProfileIdx
	}
}

const NotFoundIndex = 9999 // not found is bad

func appIndex(keys []string, key string) int {
	for idx, each := range keys {
		if strings.HasPrefix(key, each) {
			return idx
		}
	}
	return NotFoundIndex
}

func profileIndex(keys []string, key string) int {
	for idx, each := range keys {
		if strings.Contains(key, "-"+each+".") {
			return idx
		}
	}
	return NotFoundIndex
}
