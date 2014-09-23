// Copyright (c) 2013-2014 by Michael Dvorkin. All Rights Reserved.
// Use of this source code is governed by a MIT-style license that can
// be found in the LICENSE file.

package donna

func (e *Evaluation) analyzeSafety() {
	var cover, safety Total
	color := e.position.color

	// Any pawn move invalidates king's square in the pawns hash so that we
	// could detect it here.
	whiteKingMoved := e.position.king[White] != e.pawns.king[White]
	blackKingMoved := e.position.king[Black] != e.pawns.king[Black]

	if engine.trace {
		defer func() {
			var his, her Score
			e.checkpoint(`+King`, Total{*his.add(cover.white).add(safety.white), *her.add(cover.black).add(safety.black)})
			e.checkpoint(`-Cover`, cover)
			e.checkpoint(`-Safety`, safety)
		}()
	}

	oppositeBishops := func(margin int) bool {
		return e.material.flags & singleBishops != 0 && margin < -onePawn / 2 && e.oppositeBishops()
	}

	// Set the initial value of king's cover by checking his proximity from
	// friendly pawns.
	if whiteKingMoved {
		cover.white.endgame = e.kingPawnProximity(White)
		e.pawns.king[White] = e.position.king[White]
	}
	if blackKingMoved {
		cover.black.endgame = e.kingPawnProximity(Black)
		e.pawns.king[Black] = e.position.king[Black]
	}

	// Check white king's cover and safety.
	if e.material.flags & whiteKingSafety != 0 {
		if whiteKingMoved {
			e.pawns.cover[White] = e.kingCover(White)
		}
		cover.white.add(e.pawns.cover[White])

		safety.white = e.kingSafety(White)
		if oppositeBishops(cover.white.midgame + safety.white.midgame) {
			safety.white.midgame -= bishopDanger.midgame
		}

		// Apply weights by mapping White to our king safety index [3],
		// and Black to enemy's king safety index [4].
		cover.white.apply(weights[3+color])
		safety.white.apply(weights[3+color])

		e.score.add(cover.white).add(safety.white)
	}

	// Check black king's cover and safety.
	if e.material.flags & blackKingSafety != 0 {
		if blackKingMoved {
			e.pawns.cover[Black] = e.kingCover(Black)
		}
		cover.black.add(e.pawns.cover[Black])

		safety.black = e.kingSafety(Black)
		if oppositeBishops(cover.black.midgame + safety.black.midgame) {
			safety.black.midgame -= bishopDanger.midgame
		}

		// Apply weights by mapping Black to our king safety index [3],
		// and White to enemy's king safety index [4].
		cover.black.apply(weights[4-color])
		safety.black.apply(weights[4-color])

		e.score.subtract(cover.black).subtract(safety.black)
	}
}

func (e *Evaluation) kingSafety(color int) (score Score) {
	p := e.position

	if e.safety[color].threats > 0 {
		square := p.king[color]
		safetyIndex := 0

		// Find squares around the king that are being attacked by the
		// enemy and defended by our king only.
		defended := e.attacks[pawn(color)] | e.attacks[knight(color)] |
		            e.attacks[bishop(color)] | e.attacks[rook(color)] |
		            e.attacks[queen(color)]
		weak := e.attacks[king(color)] & e.attacks[color^1] & ^defended

		// Find possible queen checks on weak squares around the king.
		// We only consider squares where the queen is protected and
		// can't be captured by the king.
		protected := e.attacks[pawn(color^1)] | e.attacks[knight(color^1)] |
		             e.attacks[bishop(color^1)] | e.attacks[rook(color^1)] |
		             e.attacks[king(color^1)]
		checks := weak & e.attacks[queen(color^1)] & protected & ^p.outposts[color^1]
		if checks != 0 {
			safetyIndex += bonusCloseCheck[Queen/2] * checks.count()
		}

		// Find possible rook checks within king's home zone. Unlike
		// queen we must only consider squares where the rook actually
		// gives a check.
		protected = e.attacks[pawn(color^1)] | e.attacks[knight(color^1)] |
		            e.attacks[bishop(color^1)] | e.attacks[queen(color^1)] |
		            e.attacks[king(color^1)]
		checks = weak & e.attacks[rook(color^1)] & protected & ^p.outposts[color^1]
		checks &= rookMagicMoves[square][0]
		if checks != 0 {
			safetyIndex += bonusCloseCheck[Rook/2] * checks.count()
		}

		// Double safety index if the enemy has right to move.
		if p.color == color^1 {
			safetyIndex *= 2
		}

		// Out of all squares available for enemy pieces select the ones
		// that are not under our attack.
		safe := ^(e.attacks[color] | p.outposts[color^1])

		// Are there any safe squares from where enemy Knight could give
		// us a check?
		if checks := knightMoves[square] & safe & e.attacks[knight(color^1)]; checks != 0 {
			safetyIndex += bonusDistanceCheck[Knight/2] * checks.count()
		}

		// Are there any safe squares from where enemy Bishop could give
		// us a check?
		safeBishopMoves := p.bishopMoves(square) & safe
		if checks := safeBishopMoves & e.attacks[bishop(color^1)]; checks != 0 {
			safetyIndex += bonusDistanceCheck[Bishop/2] * checks.count()
		}

		// Are there any safe squares from where enemy Rook could give
		// us a check?
		safeRookMoves := p.rookMoves(square) & safe
		if checks := safeRookMoves & e.attacks[rook(color^1)]; checks != 0 {
			safetyIndex += bonusDistanceCheck[Rook/2] * checks.count()
		}

		// Are there any safe squares from where enemy Queen could give
		// us a check?
		if checks := (safeBishopMoves | safeRookMoves) & e.attacks[queen(color^1)]; checks != 0 {
			safetyIndex += bonusDistanceCheck[Queen/2] * checks.count()
		}

		threatIndex := Min(12, e.safety[color].attackers * e.safety[color].threats / 3) + (e.safety[color].attacks + weak.count()) * 2
		safetyIndex = Min(63, safetyIndex + threatIndex)

		score.midgame -= kingSafety[safetyIndex]
		score.endgame -= bonusKing[1][Flip(color, square)]
	}

	return
}

func (e *Evaluation) kingCover(color int) (bonus Score) {
	p, square := e.position, e.position.king[color]

	// Calculate relative square for the king so we could treat black king
	// as white. Don't bother with the cover if the king is too far.
	relative := Flip(color^1, square)
	if relative > H3 {
		return
	}

	// If we still have castle rights encourage castle pawns to stay intact
	// by scoring least safe castle.
	bonus.midgame = e.kingCoverBonus(color, square, relative)
	if p.castles & castleKingside[color] != 0 {
		bonus.midgame = Max(bonus.midgame, e.kingCoverBonus(color, homeKing[color] + 2, G1))
	}
	if p.castles & castleQueenside[color] != 0 {
		bonus.midgame = Max(bonus.midgame, e.kingCoverBonus(color, homeKing[color] - 2, C1))
	}

	return
}

func (e *Evaluation) kingCoverBonus(color, square, relative int) (bonus int) {
	row, col := Coordinate(relative)
	from, to := Max(0, col-1), Min(7, col+1)
	bonus = onePawn + onePawn / 3

	// Get friendly pawns adjacent and in front of the king.
	adjacent := maskIsolated[col] & maskRank[Row(square)]
	pawns := e.position.outposts[pawn(color)] & (adjacent | maskPassed[color][square])

	// For each of the cover files find the closest friendly pawn. The penalty
	// is carried if the pawn is missing or is too far from the king (more than
	// one rank apart).
	for column := from; column <= to; column++ {
		if cover := (pawns & maskFile[column]); cover != 0 {
			closest := RelRow(cover.closest(color), color)
			bonus -= penaltyCover[closest - row]
		} else {
			bonus -= coverMissing.midgame
		}
	}

	// Log("penalty[%s] => %+v\n", C(color), penalty)
	return
}

// Calculates endgame penalty to encourage a king stay closer to friendly pawns.
func (e *Evaluation) kingPawnProximity(color int) (penalty int) {
	if pawns := e.position.outposts[pawn(color)]; pawns != 0 && pawns & e.attacks[king(color)] == 0 {
		proximity, king := 8, e.position.king[color]

		for pawns != 0 {
			proximity = Min(proximity, distance[king][pawns.pop()])
		}
		penalty = -kingByPawn.endgame * (proximity - 1)
	}

	return
}

