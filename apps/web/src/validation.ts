export const LEARNER_42_EMAIL_PATTERN = /^[^\s@]+@learner\.42\.tech$/i
export const LEARNER_42_EMAIL_HINT = 'Use your 42 email ending in @learner.42.tech.'

export function isLearner42Email(email: string) {
  return LEARNER_42_EMAIL_PATTERN.test(email.trim())
}
