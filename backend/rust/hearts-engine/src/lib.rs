#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum HeartsVariant {
    Standard,
    Omnibus,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct RoundInput {
    /// Number of hearts captured by this player in the round.
    pub hearts_taken: u8,
    /// Whether this player captured the queen of spades.
    pub queen_of_spades: bool,
    /// Whether this player captured the jack of diamonds.
    pub jack_of_diamonds: bool,
}

impl RoundInput {
    pub fn new(hearts_taken: u8, queen_of_spades: bool, jack_of_diamonds: bool) -> Self {
        Self {
            hearts_taken,
            queen_of_spades,
            jack_of_diamonds,
        }
    }
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct RoundResult {
    pub scores: Vec<i16>,
    pub shooter_index: Option<usize>,
}

/// Calculates round scores for Hearts and detects shoot-the-moon.
///
/// Input must represent all players in seat order.
pub fn score_round(players: &[RoundInput], variant: HeartsVariant) -> Result<RoundResult, String> {
    if players.len() < 3 || players.len() > 6 {
        return Err("hearts supports between 3 and 6 players".to_owned());
    }

    let total_hearts: u16 = players.iter().map(|p| p.hearts_taken as u16).sum();
    if total_hearts != 13 {
        return Err("round must account for exactly 13 hearts".to_owned());
    }

    let queen_count = players.iter().filter(|p| p.queen_of_spades).count();
    if queen_count != 1 {
        return Err("round must contain exactly one queen of spades capture".to_owned());
    }

    if let HeartsVariant::Omnibus = variant {
        let jack_count = players.iter().filter(|p| p.jack_of_diamonds).count();
        if jack_count > 1 {
            return Err("round cannot contain multiple jack of diamonds captures".to_owned());
        }
    }

    let mut scores = vec![0_i16; players.len()];
    for (idx, player) in players.iter().enumerate() {
        let mut value = player.hearts_taken as i16;
        if player.queen_of_spades {
            value += 13;
        }
        if variant == HeartsVariant::Omnibus && player.jack_of_diamonds {
            value -= 10;
        }
        scores[idx] = value;
    }

    // In both standard and omnibus play, shoot-the-moon is based on
    // collecting all hearts and the queen of spades.
    let shooter_index = players
        .iter()
        .position(|p| p.hearts_taken == 13 && p.queen_of_spades);

    if let Some(shooter) = shooter_index {
        for (idx, score) in scores.iter_mut().enumerate() {
            *score = if idx == shooter { 0 } else { 26 };
        }
    }

    Ok(RoundResult {
        scores,
        shooter_index,
    })
}

#[cfg(test)]
mod tests {
    use super::{score_round, HeartsVariant, RoundInput};

    #[test]
    fn scores_standard_round() {
        let round = [
            RoundInput::new(4, false, false),
            RoundInput::new(2, false, false),
            RoundInput::new(7, true, false),
        ];

        let result = score_round(&round, HeartsVariant::Standard).expect("expected score");
        assert_eq!(result.scores, vec![4, 2, 20]);
        assert_eq!(result.shooter_index, None);
    }

    #[test]
    fn scores_omnibus_with_jack_bonus() {
        let round = [
            RoundInput::new(5, true, false),
            RoundInput::new(3, false, true),
            RoundInput::new(5, false, false),
        ];

        let result = score_round(&round, HeartsVariant::Omnibus).expect("expected score");
        assert_eq!(result.scores, vec![18, -7, 5]);
    }

    #[test]
    fn applies_shoot_the_moon() {
        let round = [
            RoundInput::new(13, true, false),
            RoundInput::new(0, false, false),
            RoundInput::new(0, false, false),
            RoundInput::new(0, false, false),
        ];

        let result = score_round(&round, HeartsVariant::Standard).expect("expected score");
        assert_eq!(result.shooter_index, Some(0));
        assert_eq!(result.scores, vec![0, 26, 26, 26]);
    }

    #[test]
    fn rejects_invalid_heart_count() {
        let round = [
            RoundInput::new(3, false, false),
            RoundInput::new(3, false, false),
            RoundInput::new(3, true, false),
            RoundInput::new(3, false, false),
        ];

        let result = score_round(&round, HeartsVariant::Standard);
        assert!(result.is_err());
        assert_eq!(
            result.unwrap_err(),
            "round must account for exactly 13 hearts"
        );
    }

    #[test]
    fn rejects_too_few_players() {
        let round = [
            RoundInput::new(7, false, false),
            RoundInput::new(6, true, false),
        ];

        let result = score_round(&round, HeartsVariant::Standard);
        assert!(result.is_err());
        assert_eq!(
            result.unwrap_err(),
            "hearts supports between 3 and 6 players"
        );
    }

    #[test]
    fn rejects_too_many_players() {
        let round = [
            RoundInput::new(2, false, false),
            RoundInput::new(2, false, false),
            RoundInput::new(2, false, false),
            RoundInput::new(2, false, false),
            RoundInput::new(2, true, false),
            RoundInput::new(2, false, false),
            RoundInput::new(1, false, false),
        ];

        let result = score_round(&round, HeartsVariant::Standard);
        assert!(result.is_err());
        assert_eq!(
            result.unwrap_err(),
            "hearts supports between 3 and 6 players"
        );
    }

    #[test]
    fn accepts_six_players_max() {
        let round = [
            RoundInput::new(3, false, false),
            RoundInput::new(2, false, false),
            RoundInput::new(2, false, false),
            RoundInput::new(2, true, false),
            RoundInput::new(2, false, false),
            RoundInput::new(2, false, false),
        ];

        let result = score_round(&round, HeartsVariant::Standard).expect("expected score");
        assert_eq!(result.scores, vec![3, 2, 2, 15, 2, 2]);
        assert_eq!(result.shooter_index, None);
    }

    #[test]
    fn rejects_no_queen_of_spades_captured() {
        let round = [
            RoundInput::new(5, false, false),
            RoundInput::new(4, false, false),
            RoundInput::new(4, false, false),
        ];

        let result = score_round(&round, HeartsVariant::Standard);
        assert!(result.is_err());
        assert_eq!(
            result.unwrap_err(),
            "round must contain exactly one queen of spades capture"
        );
    }

    #[test]
    fn rejects_multiple_queen_captures() {
        let round = [
            RoundInput::new(5, true, false),
            RoundInput::new(4, true, false),
            RoundInput::new(4, false, false),
        ];

        let result = score_round(&round, HeartsVariant::Standard);
        assert!(result.is_err());
        assert_eq!(
            result.unwrap_err(),
            "round must contain exactly one queen of spades capture"
        );
    }

    #[test]
    fn rejects_multiple_jack_captures_in_omnibus() {
        let round = [
            RoundInput::new(5, true, true),
            RoundInput::new(4, false, true),
            RoundInput::new(4, false, false),
        ];

        let result = score_round(&round, HeartsVariant::Omnibus);
        assert!(result.is_err());
        assert_eq!(
            result.unwrap_err(),
            "round cannot contain multiple jack of diamonds captures"
        );
    }

    #[test]
    fn allows_multiple_jack_flags_in_standard_variant() {
        // Standard variant does not validate jack of diamonds at all; extra flags are ignored.
        let round = [
            RoundInput::new(5, true, true),
            RoundInput::new(4, false, true),
            RoundInput::new(4, false, false),
        ];

        let result = score_round(&round, HeartsVariant::Standard).expect("expected score");
        // jack_of_diamonds has no effect in Standard
        assert_eq!(result.scores, vec![18, 4, 4]);
    }

    #[test]
    fn jack_of_diamonds_is_ignored_in_standard_variant() {
        // Score with and without jack flag should be identical in Standard.
        let without_jack = [
            RoundInput::new(6, true, false),
            RoundInput::new(4, false, false),
            RoundInput::new(3, false, false),
        ];
        let with_jack = [
            RoundInput::new(6, true, false),
            RoundInput::new(4, false, true),
            RoundInput::new(3, false, false),
        ];

        let result_without =
            score_round(&without_jack, HeartsVariant::Standard).expect("expected score");
        let result_with =
            score_round(&with_jack, HeartsVariant::Standard).expect("expected score");

        assert_eq!(result_without.scores, result_with.scores);
    }

    #[test]
    fn shoot_the_moon_by_last_player() {
        let round = [
            RoundInput::new(0, false, false),
            RoundInput::new(0, false, false),
            RoundInput::new(0, false, false),
            RoundInput::new(13, true, false),
        ];

        let result = score_round(&round, HeartsVariant::Standard).expect("expected score");
        assert_eq!(result.shooter_index, Some(3));
        assert_eq!(result.scores, vec![26, 26, 26, 0]);
    }

    #[test]
    fn no_shoot_the_moon_when_all_hearts_but_no_queen() {
        // Player has all 13 hearts but queen was taken by someone else.
        let round = [
            RoundInput::new(13, false, false),
            RoundInput::new(0, true, false),
            RoundInput::new(0, false, false),
        ];

        let result = score_round(&round, HeartsVariant::Standard).expect("expected score");
        assert_eq!(result.shooter_index, None);
        assert_eq!(result.scores, vec![13, 13, 0]);
    }

    #[test]
    fn omnibus_shoot_the_moon_ignores_jack_bonus() {
        // Shoot-the-moon overrides all scoring; jack bonus should not apply.
        let round = [
            RoundInput::new(13, true, true),
            RoundInput::new(0, false, false),
            RoundInput::new(0, false, false),
        ];

        let result = score_round(&round, HeartsVariant::Omnibus).expect("expected score");
        assert_eq!(result.shooter_index, Some(0));
        // After shoot-the-moon, shooter gets 0 and others get 26 regardless of jack.
        assert_eq!(result.scores, vec![0, 26, 26]);
    }

    #[test]
    fn omnibus_no_jack_captured_scores_normally() {
        // Omnibus round where no player has the jack (jack_count == 0 is allowed).
        let round = [
            RoundInput::new(7, true, false),
            RoundInput::new(3, false, false),
            RoundInput::new(3, false, false),
        ];

        let result = score_round(&round, HeartsVariant::Omnibus).expect("expected score");
        assert_eq!(result.scores, vec![20, 3, 3]);
        assert_eq!(result.shooter_index, None);
    }
}
