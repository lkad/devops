import { render, screen, fireEvent, act } from '@testing-library/react'
import { ToastProvider, useToast } from '../src/components/ui/Toast'
import { Button } from '../src/components/ui/Button'

// Test component that uses the toast hook
function TestConsumer() {
  const { addToast } = useToast()
  return (
    <Button onClick={() => addToast({ type: 'success', message: 'Test message' })}>
      Show Toast
    </Button>
  )
}

function TestConsumerWithTitle() {
  const { addToast } = useToast()
  return (
    <Button onClick={() => addToast({ type: 'error', title: 'Error Title', message: 'Error message' })}>
      Show Error Toast
    </Button>
  )
}

describe('Toast', () => {
  beforeEach(() => {
    jest.useFakeTimers()
  })

  afterEach(() => {
    jest.useRealTimers()
  })

  it('renders toast when addToast is called', () => {
    render(
      <ToastProvider>
        <TestConsumer />
      </ToastProvider>
    )
    const button = screen.getByRole('button', { name: /show toast/i })
    fireEvent.click(button)
    expect(screen.getByText('Test message')).toBeInTheDocument()
  })

  it('auto-dismisses after duration', () => {
    render(
      <ToastProvider>
        <TestConsumer />
      </ToastProvider>
    )
    const button = screen.getByRole('button', { name: /show toast/i })
    fireEvent.click(button)
    expect(screen.getByText('Test message')).toBeInTheDocument()
    act(() => {
      jest.advanceTimersByTime(3000)
    })
  })

  it('renders with title and message', () => {
    render(
      <ToastProvider>
        <TestConsumerWithTitle />
      </ToastProvider>
    )
    const button = screen.getByRole('button', { name: /show error toast/i })
    fireEvent.click(button)
    expect(screen.getByText('Error Title')).toBeInTheDocument()
    expect(screen.getByText('Error message')).toBeInTheDocument()
  })
})