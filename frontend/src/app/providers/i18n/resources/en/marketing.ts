/** The public `/about` surface (MKT-7). English half of the parity-checked catalog — same key
 *  topology, same interpolation, same meaning as `ko/marketing.ts`, which carries the claim rules. */
export const marketing = {
  metadata: {
    title: 'What is Postpilot? | Sponsored reviews from your own experience',
    description:
      'Turn photos, videos and notes from a sponsored visit into a Naver Blog post or clip in your voice. Principles guide writing from your own material; review origins and the advertiser guide, edit, and publish yourself.',
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
    positioning: 'Your own experience. Sponsored review posts, written faster.',
    title:
      'Photos and notes from your sponsored visit, ready for Naver Blog or a clip in your voice',
    body: 'Prepare a post or clip for publication from your visit’s photos, videos and notes. AI helps with phrasing and structure under principles that always apply and call for writing only from your own material. Review sentence origins and the advertiser guide, make your edits, and publish yourself.',
    access:
      'An email address and password open an account, and you verify the address by mail before your first login. Google sign-in is also available.',
    haveAccount: 'Already have an account?',
  },
  flow: {
    title: 'How it works',
    step1: {
      title: 'Choose a channel and post type',
      body: 'Choose Naver Blog, Naver Clip or Instagram Clip, then select whether your post is sponsored.',
    },
    step2: {
      title: 'Check the advertiser guide',
      body: 'If your post is sponsored, paste the advertiser guide as text. Confirm the requirements Postpilot pulls out.',
    },
    step3: {
      title: 'Add photos or videos',
      body: 'Add your photos or videos, then choose a template, your saved post structure. You can also continue without a template.',
    },
    step4: {
      title: 'Share your experience',
      body: 'Give the post a working title and fill in each template field with what you experienced. Add a closing note if you want.',
    },
    step5: {
      title: 'Choose a voice and guidelines',
      body: 'Once you choose a voice and guidelines, Postpilot observes your photos and videos before writing. Principles that always apply help keep it from inventing experiences that aren’t in your material.',
    },
    step6: {
      title: 'Review origins and revise',
      body: 'See whether each sentence is based on your input, inferred from photos, requested by the advertiser, or added by AI. Edit sentences yourself or request an AI revision, then check the advertiser guide checklist.',
    },
    step7: {
      title: 'Copy and publish manually',
      body: 'Copy the finished post in Naver Blog format. For a clip, prepare the video and upload text, then publish manually on each service.',
    },
  },
  different: {
    title: 'What is different',
    voices: {
      title: 'Your own voice and headings',
      body: 'Your chosen voice and headings shape the account from your own material. Learning material is also kept separate for each voice.',
    },
    observation: {
      title: 'Principles always apply',
      body: 'The principles to write from your material without inventing experiences cannot be switched off. Observation and writing are separate steps.',
    },
    blocks: {
      title: 'Help readers decide and support the store',
      body: 'Keep the drawbacks you noted in polite words. We aim for an honest account that helps readers decide and supports the store, without adding praise you never gave.',
    },
    control: {
      title: 'Review the origin of each sentence',
      body: 'Distinguish meaning based on your input, inferred from photos, requested by the advertiser or added by AI. You decide which wording to keep or revise.',
    },
  },
  outputs: {
    title: 'Where the result goes',
    body: 'Prepare a Naver Blog post, or Naver Clip and Instagram Clip videos with upload text. Origin labels and technical details stay out of the copied writing. Copy the post or export the video, then publish manually on each service.',
    naver: 'Naver Blog post',
    naverClip: 'Naver Clip video and upload text',
    instaClip: 'Instagram Clip video and upload text',
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
    tagline: 'From your own experience to sponsored review posts and clips',
  },
} as const
