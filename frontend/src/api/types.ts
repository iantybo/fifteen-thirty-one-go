export type User = {
  id: number
  username: string
  created_at?: string
}

// Payment and Subscription Types

export type SubscriptionPlan = {
  id: string
  name: string
  display_name: string
  description: string
  price_cents: number
  currency: string
  billing_period: 'month' | 'year'
  stripe_price_id?: string
  features: string[]
  is_active: boolean
  created_at: string
  updated_at: string
}

export type UserSubscription = {
  id: string
  user_id: number
  plan_id: string
  stripe_subscription_id?: string
  stripe_customer_id?: string
  status: 'active' | 'canceled' | 'past_due' | 'trialing' | 'incomplete'
  current_period_start: string
  current_period_end: string
  cancel_at_period_end: boolean
  canceled_at?: string
  trial_end?: string
  created_at: string
  updated_at: string
}

export type UserSubscriptionWithPlan = UserSubscription & {
  plan?: SubscriptionPlan
}

export type PaymentMethod = {
  id: string
  user_id: number
  stripe_payment_method_id: string
  stripe_customer_id: string
  type: 'card' | 'bank_account'
  card_brand?: string
  card_last4?: string
  card_exp_month?: number
  card_exp_year?: number
  is_default: boolean
  created_at: string
  updated_at: string
}

export type PaymentTransaction = {
  id: string
  user_id: number
  subscription_id?: string
  stripe_payment_intent_id?: string
  stripe_invoice_id?: string
  amount_cents: number
  currency: string
  status: 'succeeded' | 'pending' | 'failed' | 'refunded'
  description?: string
  failure_code?: string
  failure_message?: string
  receipt_url?: string
  created_at: string
  updated_at: string
}

export type CreateSubscriptionRequest = {
  plan_id: string
  payment_method_id: string
}

export type UpdatePaymentMethodRequest = {
  payment_method_id: string
}

export type CancelSubscriptionRequest = {
  cancel_at_period_end: boolean
}

export type SetupIntentResponse = {
  client_secret: string
  customer_id: string
}

export type AuthResponse = {
  user: User
}

export type Lobby = {
  id: number
  name: string
  host_id: number
  max_players: number
  current_players: number
  status: 'waiting' | 'in_progress' | 'finished'
  created_at: string
}

export type Game = {
  id: number
  lobby_id: number
  status: 'waiting' | 'in_progress' | 'finished'
  current_player_id?: number
  dealer_id?: number
  created_at: string
  finished_at?: string
}

export type GamePlayer = {
  game_id: number
  user_id: number
  username: string
  position: number
  score: number
  hand: string
  hand_count?: number
  crib_cards?: string
  is_bot: boolean
  bot_difficulty?: string
}

export type UserStats = {
  user_id: number
  games_played: number
  games_won: number
}

export type LobbyChatMessage = {
  id: number
  lobby_id: number
  user_id?: number | null
  username: string
  message: string
  message_type: 'chat' | 'system' | 'join' | 'leave'
  created_at: string
}

export type SpectatorInfo = {
  user_id: number
  username: string
  joined_at: string
  avatar_url?: string
}

export type PresenceStatus = {
  user_id: number
  username: string
  status: 'online' | 'away' | 'in_game' | 'offline'
  last_active: string
  current_lobby_id?: number
  avatar_url?: string
}

export type LeaderboardDayPoint = {
  date: string // YYYY-MM-DD
  games_played: number
  games_won: number
  win_rate: number // cumulative within the window [0..1]
}

export type LeaderboardPlayer = {
  user_id: number
  username: string
  games_played: number
  games_won: number
  win_rate: number // all-time [0..1]
  series: LeaderboardDayPoint[]
}

export type LeaderboardResponse = {
  days: number
  items: LeaderboardPlayer[]
}

export type Card = {
  rank: number
  suit: 'S' | 'H' | 'D' | 'C'
}

export type CribbageRules = {
  max_players: number
}

export type CribbageStage = 'dealing' | 'discard' | 'pegging' | 'counting' | 'finished'

export type CribbageState = {
  rules: CribbageRules
  dealer_index: number
  current_index: number
  last_play_index: number
  cut?: Card
  hands: Card[][] // other players are [] (hidden); your hand is populated
  kept_hands?: Card[][] // revealed during counting/finished
  crib?: Card[] // revealed during counting/finished
  pegging_total: number
  pegging_seq: Card[]
  pegging_passed: boolean[]
  discard_completed: boolean[]
  ready_next_hand?: boolean[]
  scores: number[]
  stage: CribbageStage
  count_summary?: {
    order: number[]
    hands?: Record<
      string,
      { total: number; fifteens: number; pairs: number; runs: number; flush: number; nobs: number; reasons?: Record<string, number> }
    >
    crib?: { total: number; fifteens: number; pairs: number; runs: number; flush: number; nobs: number; reasons?: Record<string, number> }
  }

  history?: Array<{
    round: number
    dealer_index: number
    cut?: Card
    hands?: Record<
      string,
      { total: number; fifteens: number; pairs: number; runs: number; flush: number; nobs: number; reasons?: Record<string, number> }
    >
    crib?: { total: number; fifteens: number; pairs: number; runs: number; flush: number; nobs: number; reasons?: Record<string, number> }
    scores_before?: number[]
    scores_after?: number[]
  }>
}

export type GameSnapshot = {
  game: Game
  players: GamePlayer[]
  state: CribbageState
}

export type GameMove = {
  id: number
  game_id: number
  player_id: number
  move_type: string
  card_played?: string
  score_claimed?: number
  score_verified?: number
  is_corrected: boolean
  created_at: string
}

export type ChatbotMessage = {
  id: string
  role: 'user' | 'assistant'
  content: string
  timestamp: string
}

export type ChatbotRequest = {
  message: string
  game_context?: {
    game_id: number
    stage: string
    scores: number[]
    hand_size: number
  }
}

export type ChatbotResponse = {
  message: string
  timestamp: string
}

