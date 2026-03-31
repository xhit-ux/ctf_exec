import React from 'react'

type Props = {
  children: React.ReactNode
}

type State = {
  hasError: boolean
}

class AppErrorBoundary extends React.Component<Props, State> {
  constructor(props: Props) {
    super(props)
    this.state = { hasError: false }
  }

  static getDerivedStateFromError(): State {
    return { hasError: true }
  }

  componentDidCatch(error: Error) {
    // 避免将错误吞掉，保留控制台栈用于调试。
    console.error('AppErrorBoundary caught error:', error)
  }

  handleReload = () => {
    window.location.reload()
  }

  render() {
    if (!this.state.hasError) {
      return this.props.children
    }

    return (
      <div className="app-error-boundary">
        <div className="app-error-boundary__card">
          <h1>页面出现异常</h1>
          <p>应用遇到未处理错误，建议刷新页面后重试。</p>
          <button type="button" onClick={this.handleReload}>刷新页面</button>
        </div>
      </div>
    )
  }
}

export default AppErrorBoundary
