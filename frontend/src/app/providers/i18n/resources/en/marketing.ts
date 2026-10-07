/** The public `/about` surface (MKT-7). English half of the parity-checked catalog — same key
 *  topology, same interpolation, same meaning as `ko/marketing.ts`, which carries the claim rules. */
export const marketing = {
  metadata: {
    title: 'What is Postpilot? | Blog drafts from photos and rough notes',
    description:
      'Turn photos and rough notes into a blog draft in your chosen voice. AI helps with phrasing and structure; you review meaning based on your input, inferred from photos or added by AI, edit it, and publish manually.',
  },
  about: {
    link: 'What is Postpilot?',
  },
  header: {
    nav: 'About',
    getStarted: 'Get started',
    login: 'Log in',
  },
  hero: {
    positioning: 'AI helps you move faster. You stay in control of your writing.',
    title: 'Photos and rough notes into a blog draft in your own voice',
    body: 'Turn your photos and a few lines of notes into a blog draft in your own voice. AI helps with phrasing and structure; you inspect where a phrase’s meaning came from, make your own edits, and publish it yourself.',
    access:
      'An email address and password open an account, and you verify the address by mail before your first login. Google sign-in is also available.',
    haveAccount: 'Already have an account?',
  },
  flow: {
    title: 'How it works',
    step1: {
      title: 'Pick a voice, a template, and the output language',
      body: 'For each post you choose which voice writes it, which template shapes it (or none), and whether it comes out in Korean or English.',
    },
    step2: {
      title: 'Add photos and rough notes',
      body: 'Photos are converted in your browser before they are uploaded. The notes do not have to be sentences.',
    },
    step3: {
      title: 'Observe the photos first, then write',
      body: 'AI helps with phrasing and structure using the photo observations. You pick the model for observation and for writing separately.',
    },
    step4: {
      title: 'Review origins, edit, and publish yourself',
      body: 'Review whether a phrase’s meaning is based on your input, inferred from photos or added by AI. Edit the sentences yourself or request an AI revision, then copy the finalized post in your platform’s format and publish manually on the destination service.',
    },
  },
  different: {
    title: 'What is different',
    voices: {
      title: 'Voices never bleed into each other',
      body: 'Each voice profile is learned on its own. Material collected for one voice does not turn up in another voice’s sentences.',
    },
    observation: {
      title: 'Observation is separate from writing',
      body: 'Photo observation and writing are separate steps. In the draft, you can inspect each phrase’s links to your input or photo observations and the meaning added by AI.',
    },
    blocks: {
      title: 'The post is stored as structured blocks',
      body: 'Prose, headings, images, quotes and lists are stored as one canonical structure, and every platform format is derived from it.',
    },
    control: {
      title: 'You choose the models and the runs',
      body: 'You pick which model runs each step and decide when to compare drafts or request an AI revision.',
    },
  },
  outputs: {
    title: 'Where the result goes',
    body: 'One finalized post produces every format below. Origin labels and technical details stay out of the copied writing. You manually publish the copied result on the destination service.',
    naver: 'Naver Blog',
    tistory: 'Tistory',
    html: 'HTML for your own site',
    markdown: 'Markdown',
  },
  plans: {
    title: 'Plans',
    body: 'Plans differ in daily credits, monthly bonuses, model access and included server exports. Choose and pay for a plan on the Plans screen.',
    assignment:
      'Subscribers choose a tier on the Plans screen and pay there to start a subscription.',
    master:
      'master is the operator tier. It has no usage limits and owns account administration. It is not a tier a user can be given.',
  },
  facts: {
    title: 'Control and data',
    images: 'Original photos are converted in your browser before upload.',
    isolation: 'Learning material is kept separate per account and per voice.',
    noBackground: 'Opening a screen never starts AI work. Every run is something you press.',
    credentials: 'Copying and exporting require no credentials for a destination service.',
  },
  footer: {
    tagline: 'From photos and notes to a blog draft',
  },
} as const
