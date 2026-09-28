export default function Footer() {
  return (
    <footer className="border-t">
      <div className="mx-auto flex max-w-[1280px] flex-wrap items-center justify-between gap-2 px-4 md:px-6 py-4 text-xs text-muted-foreground">
        <span>Nanit Dashboard</span>
        <span>
          Not affiliated with Nanit. API reverse engineering by{' '}
          <a
            href="https://github.com/indiefan/home_assistant_nanit"
            target="_blank"
            rel="noopener noreferrer"
            className="text-ctp-blue-700 dark:text-ctp-blue hover:underline"
          >
            indiefan/home_assistant_nanit
          </a>
          .
        </span>
      </div>
    </footer>
  )
}
