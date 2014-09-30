package orgreminders

import (
    "fmt"
    "regexp"
    "strconv"
    "time"
)

type Schedule struct {
    Name string
    When []string
}

// Generates a new alert schedule.
func NewSchedule(n string) Schedule {
    return Schedule{ Name: n, }
}

// Add a reminder offset to the schedule.
func (s *Schedule) Add(when string) {
    s.When = append(s.When, when)
}

// Delete a reminder offset from the schedule.
func (s *Schedule) Del(when string) {
    var newWhen = []string{}

    for _, val := range s.When {
        if val != when {
            newWhen = append(newWhen, val)
        }
    }

    s.When = newWhen
}

// Return times (in current Locale) of the whole schedule.
func (s *Schedule) Times(baseTime time.Time) map[string]time.Time {
    var times = make(map[string]time.Time)
    var timeNil = time.Date(0, 0, 0, 0, 0, 0, 0, time.UTC)
    reI := regexp.MustCompile(`\d+`)
    reS := regexp.MustCompile(`\D`)
    for _, val := range s.When {
        var valTime = baseTime
        var unitVal = reS.FindString(val)
        var offsetVal = reI.FindString(val)

        // Get our integer offset value
        intVal,err := strconv.Atoi(offsetVal)
        if err != nil {
            fmt.Println("ERR: cannot convert " + offsetVal + " to integer... skipping")
            continue
        }
        intValD := time.Duration(intVal)

        if unitVal == "m" {
            valTime = valTime.Add(-1 * intValD * time.Minute)
        } else if unitVal == "h" {
            valTime = valTime.Add(-1 * intValD * time.Hour)
        } else if unitVal == "d" {
            valTime = valTime.Add(-24 * intValD * time.Hour)
        } else if unitVal == "w" {
            valTime = valTime.Add(-168 * intValD * time.Hour)
        } else       {
            valTime = timeNil
        }

        times[val] = valTime
    }

    return times
}

// Get schedule in usable format for HTML
func (s *Schedule) HTML() map[string][]string {
    var result = make(map[string][]string)

    reI := regexp.MustCompile(`\d+`)
    reS := regexp.MustCompile(`\D`)
    for _, val := range s.When {
        var unitVal = reS.FindString(val)
        var offsetVal = reI.FindString(val)
        var vals = []string{offsetVal, unitVal}

        result[val] = vals
    }

    return result
}
