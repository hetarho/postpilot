export const auth = {
  login: {
    intro: 'Log in to continue',
    id: 'Login ID',
    password: 'Password',
    submit: 'Log in',
    failed: 'The login ID or password is incorrect',
  },
  logout: {
    failed: 'Could not log out. Your session is still active, so please try again.',
  },
  account: {
    label: 'My account',
    signedInAs: 'Signed in as',
  },
} as const
