/** The public `/about` surface (plan 15). English half of the parity-checked catalog — same key
 *  topology, same interpolation, same meaning as `ko/marketing.ts`, which carries the claim rules. */
export const marketing = {
  metadata: {
    title: 'What is Postpilot? | Blog drafts from photos and rough notes',
    description:
      'Turn photos and rough notes into a blog draft in a voice you trained. Photo observation is separated from writing, and you pick the AI model for each step.',
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
    title: 'Photos and rough notes into a blog draft in your own voice',
    body: 'Upload the photos you took and a few lines of notes, and Postpilot writes a draft you can paste into your blog, in a voice profile you trained. Reading it and making one pass of edits is part of the same flow.',
    access:
      'An email address and password open an account, and you verify the address by mail before your first login. Google sign-in is also available.',
  },
  flow: {
    title: 'How it works',
    step1: {
      title: 'Pick a voice, a purpose, and the output language',
      body: 'For each post you choose which voice writes it, what kind of text it is (its purpose), and whether it comes out in Korean or English.',
    },
    step2: {
      title: 'Add photos and rough notes',
      body: 'Photos are converted in your browser before they are uploaded. The notes do not have to be sentences.',
    },
    step3: {
      title: 'Observe the photos first, then write',
      body: 'Postpilot first records what the photos actually show, then writes the prose from that. You pick the model for the observation step and for the writing step separately.',
    },
    step4: {
      title: 'Revise, finalize, and export',
      body: 'Ask for the change you want and only that part is rewritten; then copy the finalized post in the format your platform wants.',
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
      body: 'Recording what the photos show is a distinct step from writing the post, which leaves less room for inventing things the photos never showed.',
    },
    blocks: {
      title: 'The post is stored as structured blocks',
      body: 'Prose, headings, images, quotes and lists are stored as one canonical structure, and every platform format is derived from it.',
    },
    control: {
      title: 'You choose the models and the runs',
      body: 'You pick which model runs each step, and comparing two models side by side or running a revision are always explicit actions you take.',
    },
  },
  outputs: {
    title: 'Where the result goes',
    body: 'One finalized post produces every format below. Nothing is rewritten per format.',
    naver: 'Naver Blog',
    tistory: 'Tistory',
    html: 'HTML for your own site',
    markdown: 'Markdown',
    publishing:
      'Automated Naver publishing is a separate action you trigger for a finalized post, carried out by a paired Mac. It is currently an operator-tier surface and its live verification is still in progress.',
  },
  plans: {
    title: 'Plans',
    body: 'A plan decides one thing: how many credits you receive each month. AI work is counted in credits, and every plan can choose from the same models.',
    assignment:
      'Subscribers choose a tier on the Plans screen and pay there to start a subscription.',
    columns: {
      plan: 'Plan',
      monthlyCredits: 'Credits a month',
      price: 'Monthly price',
      models: 'Models',
    },
    free: {
      name: 'free',
      monthlyCredits: '50 credits',
      price: 'Free',
      models: 'Every registered model',
    },
    basic: {
      name: 'basic',
      monthlyCredits: '220 credits',
      price: '$2',
      models: 'Every registered model',
    },
    pro: {
      name: 'pro',
      monthlyCredits: '575 credits',
      price: '$5',
      models: 'Every registered model',
    },
    max: {
      name: 'max',
      monthlyCredits: '1,200 credits',
      price: '$10',
      models: 'Every registered model',
    },
    master:
      'master is the operator tier. It has no usage limits and owns automated Naver publishing and account administration. It is not a tier a user can be given.',
  },
  facts: {
    title: 'Control and data',
    images: 'Original photos are converted in your browser before upload.',
    isolation: 'Learning material is kept separate per account and per voice.',
    noBackground: 'Opening a screen never starts AI work. Every run is something you press.',
    credentials: 'Naver credentials and browser state stay on the paired Mac.',
  },
  footer: {
    tagline: 'From photos and notes to a blog draft',
  },
} as const
