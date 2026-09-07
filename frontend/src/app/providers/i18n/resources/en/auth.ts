export const auth = {
  login: {
    intro: 'Log in to continue',
    id: 'Email or username',
    password: 'Password',
    submit: 'Log in',
    failed: 'The login ID or password is incorrect',
    passwordChanged: 'Password changed. Please log in again.',
  },
  field: {
    email: 'Email',
    password: 'Password',
  },
  links: {
    login: 'Log in',
    signup: 'Sign up',
    forgotPassword: 'Forgot password',
    more: 'Account links',
  },
  signup: {
    intro: 'Create your Postpilot account with email',
    passwordHint: 'Use at least 8 characters.',
    submit: 'Sign up',
    mailedHeading: 'Check your mail',
    mailedBody:
      'We sent a verification link to {{email}}. If the address is already registered, it receives an account notice instead.',
  },
  verification: {
    checking: 'Verifying your email',
    checkingBody: 'This will only take a moment.',
    successHeading: 'Email verified',
    successBody: 'You can log in now.',
    failedHeading: 'Could not verify your email',
    resend: 'Resend',
    resent: 'We sent another verification email to {{email}}.',
  },
  logout: {
    failed: 'Could not log out. Your session is still active, so please try again.',
  },
  account: {
    label: 'My account',
    signedInAs: 'Signed in as',
  },
  accountSettings: {
    heading: 'Account settings',
    identity: 'Account information',
    id: 'Account ID',
    noEmail: 'No email registered',
    verified: 'Verified',
    unverified: 'Verification pending',
  },
  registerEmail: {
    heading: 'Register email',
    intro: 'Add the address that should receive your verification mail.',
    submit: 'Send verification mail',
    sent: 'We sent a verification email to {{email}}.',
  },
  forgotPassword: {
    heading: 'Forgot password',
    intro: 'Enter the verified email registered to your account.',
    submit: 'Send reset mail',
    sentHeading: 'Check your mail',
    sentBody: 'If an account exists for {{email}}, we sent it a password reset link.',
  },
  resetPassword: {
    heading: 'Set a new password',
    intro: 'Enter the new password you will use to log in.',
    newPassword: 'New password',
    passwordHint: 'Use at least 8 characters.',
    submit: 'Reset password',
    successHeading: 'Password changed',
    successBody: 'Log in with your new password.',
    requestAgain: 'Request another reset mail',
  },
  changePassword: {
    heading: 'Change password',
    intro: 'Changing it logs you out on this device and every other device.',
    currentPassword: 'Current password',
    newPassword: 'New password',
    submit: 'Change password',
    noPassword:
      'This account has no current password. Log out, then set one through Forgot password.',
  },
} as const
