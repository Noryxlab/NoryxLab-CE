import * as React from 'react';
import * as TabsPrimitive from '@radix-ui/react-tabs';
import { cn } from '@/lib/utils';

export const Tabs = TabsPrimitive.Root;

export const TabsList = React.forwardRef<
  React.ComponentRef<typeof TabsPrimitive.List>,
  React.ComponentPropsWithoutRef<typeof TabsPrimitive.List>
>(function TabsList({ className, ...props }, ref) {
  return (
    <TabsPrimitive.List
      ref={ref}
      className={cn('scroll-x flex items-center gap-1 border-b border-border', className)}
      {...props}
    />
  );
});

export const TabsTrigger = React.forwardRef<
  React.ComponentRef<typeof TabsPrimitive.Trigger>,
  React.ComponentPropsWithoutRef<typeof TabsPrimitive.Trigger>
>(function TabsTrigger({ className, ...props }, ref) {
  return (
    <TabsPrimitive.Trigger
      ref={ref}
      className={cn(
        'relative whitespace-nowrap px-3 py-2 text-sm font-medium text-muted-foreground transition-colors',
        'hover:text-foreground',
        'focus-visible:outline-2 focus-visible:outline-offset-[-2px] focus-visible:outline-ring',
        'data-[state=active]:text-foreground',
        'after:absolute after:inset-x-2 after:bottom-[-1px] after:h-0.5 after:rounded-full',
        'data-[state=active]:after:bg-brand',
        className,
      )}
      {...props}
    />
  );
});

export const TabsContent = React.forwardRef<
  React.ComponentRef<typeof TabsPrimitive.Content>,
  React.ComponentPropsWithoutRef<typeof TabsPrimitive.Content>
>(function TabsContent({ className, ...props }, ref) {
  return (
    <TabsPrimitive.Content
      ref={ref}
      className={cn('pt-4 focus-visible:outline-none', className)}
      {...props}
    />
  );
});
