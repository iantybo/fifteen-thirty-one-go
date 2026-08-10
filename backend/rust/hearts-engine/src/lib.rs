#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum HeartsVariant {
    Standard,
    Omnibus,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct RoundInput {
    pub hearts_taken: u8,
    pub queen_of_spades: bool,
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

    if matches!(variant, HeartsVariant::Omnibus) {
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
        if matches!(variant, HeartsVariant::Omnibus) && player.jack_of_diamonds {
            value -= 10;
        }
        scores[idx] = value;
    }

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
}
