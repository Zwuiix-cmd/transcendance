import type { GameSettings } from './types'

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
}
