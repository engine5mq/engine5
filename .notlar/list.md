Yes, Go provides built-in support for various data structures, including lists (called slices in Go). Here's an example of how you can create a slice of `struct` instances and add items to it:

```go
package main

import "fmt"

// Define a struct.
type Person struct {
    Name string
    Age  int
}

func main() {
    // Create an empty slice of Person.
    var persons []Person

    // Add items to the slice.
    persons = append(persons, Person{"Alice", 20})
    persons = append(persons, Person{"Bob", 21})
    persons = append(persons, Person{"Charlie", 22})

    // Print the items in the slice.
    for _, person := range persons {
        fmt.Printf("Name: %s, Age: %d\n", person.Name, person.Age)
    }
}
```

In this code, `persons` is a slice of `Person` instances. We can add items to the slice using the `append` function and then print them out with a `for` loop. Note that we use `range` in the loop to iterate over each item in the slice.
