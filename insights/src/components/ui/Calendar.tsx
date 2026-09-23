import { DayPicker, type DayPickerProps } from "react-day-picker";
import "react-day-picker/style.css";
import { cn } from "../../lib/cn";

export function Calendar({ className, showOutsideDays = true, ...props }: DayPickerProps) {
  return (
    <DayPicker
      showOutsideDays={showOutsideDays}
      navLayout="after"
      className={cn("insights-calendar", className)}
      {...props}
    />
  );
}
