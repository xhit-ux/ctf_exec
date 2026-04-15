import React from 'react'
import { Link } from 'react-router-dom'

type AuthShellProps = {
  heading: string
  title: string
  description: string
  switchText: string
  switchLabel: string
  switchTo: string
  children: React.ReactNode
}

export default function AuthShell(props: AuthShellProps) {
  const { heading, title, description, switchText, switchLabel, switchTo, children } = props

  return (
    <section className="auth-shell">
      <aside className="auth-shell__hero" aria-hidden="true">
        <div className="auth-shell__hero-content">
          <p className="auth-shell__kicker">CTF Platform</p>
          <h1>{heading}</h1>
          <p>{description}</p>
        </div>
        <div className="auth-shell__hero-foot">Real Practice. Clear Progress.</div>
      </aside>

      <div className="auth-shell__form">
        <header className="auth-shell__form-header">
          <h2>{title}</h2>
        </header>

        {children}

        <div className="auth-shell__switch">
          <span>{switchText}</span>{' '}
          <Link to={switchTo}>{switchLabel}</Link>
        </div>
      </div>
    </section>
  )
}