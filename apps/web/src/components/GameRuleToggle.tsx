type GameRuleToggleProps = {
  label: string
  hint?: string
  checked: boolean
  onChange: (value: boolean) => void
}

export function GameRuleToggle({ label, hint, checked, onChange }: GameRuleToggleProps) {
  return <label className="game-rule-toggle">
    <input type="checkbox" checked={checked} onChange={(event) => onChange(event.target.checked)} />
    <span><strong>{label}</strong>{hint ? <small>{hint}</small> : null}</span>
  </label>
}
