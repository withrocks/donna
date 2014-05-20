// Copyright (c) 2013-2014 by Michael Dvorkin. All Rights Reserved.
// Use of this source code is governed by a MIT-style license that can
// be found in the LICENSE file.

package donna

func (e *Evaluator) analyzeSafety() {
	var white, black [2]Score

	if Settings.Trace {
		defer func() {
			var his, her Score
			e.checkpoint(`+King`, Total{*his.add(white[0]).add(white[1]), *her.add(black[0]).add(black[1])})
			e.checkpoint(`-Cover`, Total{white[0], black[0]})
			e.checkpoint(`-Danger`, Total{white[1], black[1]})
		}()
	}

	white[0] = e.kingCover(White)
	black[0] = e.kingCover(Black)

	white[1] = e.kingDanger(White)
	black[1] = e.kingDanger(Black)
	e.score.add(white[0]).add(white[1]).subtract(black[0]).subtract(black[1])
}

func (e *Evaluator) kingDanger(color int) (score Score) {
	square := Flip(color, e.position.king[color])

	if e.attacks[color^1] > 0 {
		score.midgame -= Max(3, e.attacks[color^1]) * e.threats[color^1]
		score.endgame -= bonusKing[1][square]
	}

	return
}

func (e *Evaluator) kingCover(color int) (penalty Score) {
	p := e.position
	kings, pawns := p.outposts[king(color)], p.outposts[pawn(color)]

	// Pass if a) the king is missing, b) the king is on the initial square
	// or c) the opposite side doesn't have a queen with one major piece.
	if kings == 0 || kings == bit[homeKing[color]] || !e.strongEnough(color^1) {
		return
	}

	// Calculate relative square for the king so we could treat black king
	// as white. Don't bother with the cover if the king is too far.
	square := Flip(color^1, p.king[color])
	if square > H3 {
		return
	}
	row, col := Coordinate(square)
	from, to := Max(0, col-1), Min(7, col+1)

	// For each of the cover columns find the closest same color pawn. The
	// penalty is carried if the pawn is missing or is too far from the king
	// (more than one row apart).
	for column := from; column <= to; column++ {
		if cover := (pawns & maskFile[column]); cover != 0 {
			closest := Flip(color^1, cover.first()) // Make it relative.
			if distance := Abs(Row(closest) - row); distance > 1 {
				penalty.midgame += distance * -coverDistance.midgame
			}
		} else {
			penalty.midgame += -coverMissing.midgame
		}
	}
	// Log("penalty[%s] => %d\n", C(color), penalty)
	return
}

