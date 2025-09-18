import { useState } from 'react'
import './App.css'

import {
  Sidebar,
  SidebarContent,
  SidebarFooter,
  SidebarGroup,
  SidebarGroupContent,
  SidebarGroupLabel,
  SidebarHeader,
  SidebarInset,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarProvider,
  SidebarTrigger,
  SidebarInput,
} from '@/components/ui/sidebar'
import { FileText, Shield, MessageSquare, ShieldCheck } from 'lucide-react'
import Logs from '@/pages/Logs'
import Policies from '@/pages/Policies'
import Playground from '@/pages/Playground'
import Guardrail from '@/pages/Guardrail'

type Tab = 'logs' | 'policies' | 'playground' | 'guardrail'

export default function App() {
  const [tab, setTab] = useState<Tab>('logs')

  return (
    <SidebarProvider>
      <Sidebar variant="sidebar" collapsible="icon">
        <SidebarHeader>
          <div className="px-2 pt-2">
            <div className="text-sm font-semibold">Flash Gateway Dash</div>
            <div className="text-xs text-muted-foreground">Demo</div>
          </div>
          <div className="px-2">
            <SidebarInput placeholder="Search…" />
          </div>
        </SidebarHeader>
        <SidebarContent>
          <SidebarGroup>
            <SidebarGroupLabel>Navigation</SidebarGroupLabel>
            <SidebarGroupContent>
              <SidebarMenu>
                <SidebarMenuItem>
                  <SidebarMenuButton tooltip="Logs" isActive={tab === 'logs'} onClick={() => setTab('logs')}>
                    <FileText className="shrink-0" />
                    <span>Logs</span>
                  </SidebarMenuButton>
                </SidebarMenuItem>
                <SidebarMenuItem>
                  <SidebarMenuButton tooltip="Policies" isActive={tab === 'policies'} onClick={() => setTab('policies')}>
                    <Shield className="shrink-0" />
                    <span>Policies</span>
                  </SidebarMenuButton>
                </SidebarMenuItem>
                <SidebarMenuItem>
                  <SidebarMenuButton tooltip="Guardrail" isActive={tab === 'guardrail'} onClick={() => setTab('guardrail')}>
                    <ShieldCheck className="shrink-0" />
                    <span>Guardrail</span>
                  </SidebarMenuButton>
                </SidebarMenuItem>
                <SidebarMenuItem>
                  <SidebarMenuButton tooltip="Playground" isActive={tab === 'playground'} onClick={() => setTab('playground')}>
                    <MessageSquare className="shrink-0" />
                    <span>Playground</span>
                  </SidebarMenuButton>
                </SidebarMenuItem>
              </SidebarMenu>
            </SidebarGroupContent>
          </SidebarGroup>
        </SidebarContent>
        <SidebarFooter>
          <div className="text-xs text-muted-foreground px-2">v0.1 • shadcn</div>
        </SidebarFooter>
      </Sidebar>
      <SidebarInset>
        <header className="flex h-12 shrink-0 items-center gap-2 border-b px-4 sticky top-0 z-10 bg-background">
          <SidebarTrigger />
          <div className="text-sm font-medium">{tab === 'logs' ? 'Logs' : tab === 'policies' ? 'Policies' : tab === 'guardrail' ? 'Guardrail' : 'Playground'}</div>
        </header>
        <div className="p-4">{tab === 'logs' ? <Logs /> : tab === 'policies' ? <Policies /> : tab === 'guardrail' ? <Guardrail /> : <Playground />}</div>
      </SidebarInset>
    </SidebarProvider>
  )
}
