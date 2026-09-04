use crate::adapter::greeting_sqlite::GreetingSqlite;

pub struct GreetingControllerImpl {
    pub store: GreetingSqlite,
}
