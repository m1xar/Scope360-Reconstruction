package helpers

import (
	"math"

	"github.com/m1xar/scope360-reconstruction/pkg/hyperliquid/connector/hyperliquid/models"
)

type FillMatch struct {
	Fills []models.RawFill
}

func MatchFillGroups(fills []models.RawFill) ([]FillMatch, map[int64]struct{}) {
	usedFills := make(map[int64]struct{})
	matched := make(map[int64]struct{})
	const sizeEpsilon = 1e-9

	matches := make([]FillMatch, 0)

	for i := 0; i < len(fills); i++ {
		f := fills[i]
		if _, ok := usedFills[f.Tid]; ok || !IsOpen(f.Dir) {
			continue
		}

		symbol := f.Coin
		side := SideFromDir(f.Dir)
		size := MustFloat(f.Sz)

		recon := []models.RawFill{f}
		usedFills[f.Tid] = struct{}{}

		for j := i + 1; j < len(fills); j++ {
			n := fills[j]
			if _, ok := usedFills[n.Tid]; ok ||
				n.Coin != symbol ||
				SideFromDir(n.Dir) != side {
				continue
			}

			sz := MustFloat(n.Sz)
			if IsOpen(n.Dir) {
				size += sz
			} else if IsClose(n.Dir) {
				size -= sz
			}

			recon = append(recon, n)
			usedFills[n.Tid] = struct{}{}

			if math.Abs(size) < sizeEpsilon {
				cp := make([]models.RawFill, len(recon))
				copy(cp, recon)

				matches = append(matches, FillMatch{Fills: cp})
				for _, fl := range cp {
					matched[fl.Tid] = struct{}{}
				}
				break
			}
		}
	}

	return matches, matched
}
