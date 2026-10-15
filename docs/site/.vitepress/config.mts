import { defineConfig } from 'vitepress'

const repo = 'https://github.com/hwhang0917/local-eml'
// The landing page (docs/intro) is served at the site root; the docs live
// under /docs/ beside it. Screenshots are referenced from the landing page
// so they exist once.
const site = 'https://hwhang0917.github.io/local-eml/'

const howto = {
  en: [
    { text: 'Open .eml files', link: '/how-to/open-eml-files' },
    { text: 'View a Google Takeout .mbox', link: '/how-to/takeout-mbox' },
    { text: 'Open a .pst without Outlook', link: '/how-to/pst-without-outlook' },
    { text: 'Free up Gmail storage', link: '/how-to/free-up-gmail' },
  ],
  ko: [
    { text: '.eml 파일 열기', link: '/ko/how-to/open-eml-files' },
    { text: 'Google 테이크아웃 .mbox 보기', link: '/ko/how-to/takeout-mbox' },
    { text: 'Outlook 없이 .pst 열기', link: '/ko/how-to/pst-without-outlook' },
    { text: 'Gmail 용량 비우기', link: '/ko/how-to/free-up-gmail' },
  ],
}

const pages = {
  en: [
    { text: 'Install', link: '/install' },
    { text: 'Importing mail', link: '/import' },
    { text: 'Searching', link: '/search' },
    { text: 'Receiving new mail', link: '/sync' },
    { text: 'Spam & phishing', link: '/spam-phishing' },
    { text: 'Privacy & your data', link: '/privacy' },
    { text: 'Settings & CLI', link: '/settings-cli' },
    { text: 'Troubleshooting', link: '/troubleshooting' },
  ],
  ko: [
    { text: '설치', link: '/ko/install' },
    { text: '메일 가져오기', link: '/ko/import' },
    { text: '검색', link: '/ko/search' },
    { text: '새 메일 받기', link: '/ko/sync' },
    { text: '스팸·피싱', link: '/ko/spam-phishing' },
    { text: '개인정보와 데이터', link: '/ko/privacy' },
    { text: '설정과 CLI', link: '/ko/settings-cli' },
    { text: '문제 해결', link: '/ko/troubleshooting' },
  ],
}

export default defineConfig({
  base: '/local-eml/docs/',
  title: 'Local Eml',
  cleanUrls: true,
  // The app's own address is a link worth keeping even though no public
  // checker can resolve it.
  ignoreDeadLinks: [/^http:\/\/localhost/],
  head: [['link', { rel: 'icon', href: '/local-eml/docs/icon.png' }]],

  locales: {
    root: {
      label: 'English',
      lang: 'en',
      description: 'A private, local viewer and archive for .eml files.',
      themeConfig: {
        nav: [
          { text: 'Home', link: site },
          { text: 'Guide', link: '/install' },
          { text: 'Releases', link: `${repo}/releases` },
        ],
        sidebar: [{ text: 'How-to', items: howto.en }, { text: 'Guide', items: pages.en }],
        editLink: { pattern: `${repo}/edit/main/docs/site/:path`, text: 'Edit this page on GitHub' },
        outline: { label: 'On this page' },
        docFooter: { prev: 'Previous', next: 'Next' },
      },
    },
    ko: {
      label: '한국어',
      lang: 'ko',
      link: '/ko/',
      description: '내 PC에서 .eml 파일을 보관하고 읽는 개인용 뷰어.',
      themeConfig: {
        nav: [
          { text: '홈', link: site },
          { text: '가이드', link: '/ko/install' },
          { text: '릴리스', link: `${repo}/releases` },
        ],
        sidebar: [{ text: '따라하기', items: howto.ko }, { text: '가이드', items: pages.ko }],
        editLink: { pattern: `${repo}/edit/main/docs/site/:path`, text: 'GitHub에서 이 페이지 수정' },
        outline: { label: '이 페이지에서' },
        docFooter: { prev: '이전', next: '다음' },
        darkModeSwitchLabel: '테마',
        sidebarMenuLabel: '메뉴',
        returnToTopLabel: '맨 위로',
        langMenuLabel: '언어',
      },
    },
  },

  themeConfig: {
    logo: '/icon.png',
    socialLinks: [{ icon: 'github', link: repo }],
    search: {
      provider: 'local',
      options: {
        locales: {
          ko: {
            translations: {
              button: { buttonText: '검색', buttonAriaLabel: '검색' },
              modal: {
                noResultsText: '결과가 없습니다',
                resetButtonTitle: '지우기',
                displayDetails: '자세히 보기',
                footer: { selectText: '선택', navigateText: '이동', closeText: '닫기' },
              },
            },
          },
        },
      },
    },
  },
})
