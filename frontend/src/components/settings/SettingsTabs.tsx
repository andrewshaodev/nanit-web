import { ReactNode } from 'react';
import type { LucideIcon } from 'lucide-react';
import { Tabs as TabsPrimitive } from 'radix-ui';

export interface SettingsTab {
  id: string;
  label: string;
  // Shown instead of label on narrow screens, where the tabs sit in a row
  shortLabel?: string;
  icon: LucideIcon;
  content: ReactNode;
  disabled?: boolean;
}

interface SettingsTabsProps {
  tabs: SettingsTab[];
  defaultTab?: string;
}

// GitHub's settings layout: a vertical menu on the left, the active item
// marked with a mauve bar, and the section on the right under a subhead.
// Built on Radix's tabs, so arrow keys move through the menu.
export default function SettingsTabs({ tabs, defaultTab }: SettingsTabsProps) {
  return (
    <TabsPrimitive.Root
      defaultValue={defaultTab || tabs[0]?.id}
      orientation="vertical"
      className="flex flex-col gap-6 md:flex-row"
    >
      <TabsPrimitive.List
        aria-label="Settings sections"
        className="flex shrink-0 gap-1 overflow-x-auto md:w-56 md:flex-col"
      >
        {tabs.map((tab) => (
          <TabsPrimitive.Trigger
            key={tab.id}
            value={tab.id}
            disabled={tab.disabled}
            className="group relative flex items-center gap-2 whitespace-nowrap rounded-md px-3 py-1.5 text-left text-muted-foreground outline-none transition-colors hover:bg-accent hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring disabled:pointer-events-none disabled:opacity-50 data-[state=active]:bg-accent data-[state=active]:font-semibold data-[state=active]:text-foreground"
          >
            <span
              className="absolute inset-y-1.5 left-0 hidden w-[3px] rounded-full bg-ctp-mauve group-data-[state=active]:md:block"
              aria-hidden="true"
            />
            <tab.icon className="size-4 shrink-0 group-data-[state=active]:text-ctp-mauve" />
            <span className="md:hidden">{tab.shortLabel ?? tab.label}</span>
            <span className="hidden md:inline">{tab.label}</span>
          </TabsPrimitive.Trigger>
        ))}
      </TabsPrimitive.List>

      {tabs.map((tab) => (
        <TabsPrimitive.Content key={tab.id} value={tab.id} className="min-w-0 flex-1 outline-none">
          <h2 className="mb-4 border-b pb-2 text-xl font-semibold">{tab.label}</h2>
          <div className="space-y-6">{tab.content}</div>
        </TabsPrimitive.Content>
      ))}
    </TabsPrimitive.Root>
  );
}
