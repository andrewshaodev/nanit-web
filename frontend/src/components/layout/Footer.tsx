import useSWR from 'swr'
import { api } from '@/lib/api'

const REPO_URL = 'https://github.com/andrewshaodev/nanit-web'

export default function Footer() {
  // The commit this build came from; local builds have none
  const { data } = useSWR('/health', () => api.getLiveness(), { revalidateOnFocus: false })
  const version = data?.version?.slice(0, 7)

  return (
    <footer className="border-t">
      <div className="mx-auto flex max-w-[1280px] flex-wrap items-center justify-between gap-2 px-4 md:px-6 py-4 text-xs text-muted-foreground">
        <span>
          <a
            href={REPO_URL}
            target="_blank"
            rel="noopener noreferrer"
            className="text-ctp-blue-700 dark:text-ctp-blue hover:underline"
          >
            Nanit Web
          </a>
          {version && (
            <>
              {' · '}
              <a
                href={`${REPO_URL}/commit/${data?.version}`}
                target="_blank"
                rel="noopener noreferrer"
                className="font-mono hover:underline"
                title="The commit this build came from"
              >
                {version}
              </a>
            </>
          )}
        </span>
        <span>Not affiliated with Nanit.</span>
      </div>
    </footer>
  )
}
