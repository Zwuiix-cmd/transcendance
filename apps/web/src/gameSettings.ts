import type { BotDifficulty, GameSettings } from './types'

export const BOT_DIFFICULTY_OPTIONS: { value: BotDifficulty; label: string; hint: string }[] = [
  { value: 'easy', label: 'Easy', hint: 'Plays random legal cards, forgets UNO, never challenges' },
  { value: 'intermediate', label: 'Intermediate', hint: 'Solid play, keeps wilds, sometimes catches you' },
  { value: 'expert', label: 'Expert', hint: 'Counts cards, blocks you, bluffs +4 and never misses UNO' },
]

export function botDifficultyLabel(value: BotDifficulty | undefined) {
  return BOT_DIFFICULTY_OPTIONS.find((option) => option.value === value)?.label ?? 'Intermediate'
}

export const CUSTOM_PLAYER_OPTIONS = [2, 3, 4, 5, 6, 7, 8]
export const CUSTOM_HAND_OPTIONS = [3, 5, 7, 9, 11, 14]
export const DRAW_PENALTY_OPTIONS = [1, 2, 3, 4]

export const DEFAULT_GAME_SETTINGS: GameSettings = {
  maxPlayers: 4,
  startingHandSize: 7,
  drawPenalty: 2,
  stackDrawCards: false,
  jumpIn: false,
  sevenZero: false,
  drawUntilPlayable: false,
  forcePlay: false,
  challengeDrawFour: true,
  allowMultipleDecks: false,
  botsEnabled: false,
  botDifficulty: 'intermediate',
}
