# goverter 非官方扩展功能

定义我们有如下类型，可以在下列的代码例子中直接使用

```go
type Account {
    id int64
    Name string
    Birthday string
}


type User {
    Name string
    Birthday string
}
```

1. #### noStrict标识
   
   noStrict标识，忽略在目标结构体中没有匹配的字段，和在目标结构体中没有到处的字段，在原goverter中，出现以上例子会中断生成代码并报错
   
   使用如下：
   
   ```go
   // 在interface使用noStrict
   
   // goverter:converter
   // goverter:noStrict
   type Converter interface {
       AccountToUser(in *Account) User
   }
   
   // 在具体方法上使用noStrict
   
   // goverter:converter
   type Converter2 interface {
       // goverter:noStrict
       AccountToUser(in *Account) User
   }
   ```

2. ##### strict标识
   
   用于取消noStrict设置，注意该标识只能在方法上使用
   
   使用如下：
   
   ```go
   // goverter:converter
   // goverter:noStrict
   type Converter interface {
       AccountToUser(in *Account) User
       // goverter:strict
       StrictAccountToUser(in *Account) User
   }
   ```

3. ##### ignoreUnexported标识
   
   该标识与noStrict配合使用，在默认的行为中，noStrict标识会对为导出的字段作警告处理，可以使用该标识改变默认的行为，该标识可以在interface与方法上使用，在interface上使用表示对该interface上的所有方法都使用该行为，在方法上表示只修改该方法的行为

4. ##### unexported标识
   
   功能同nostrict标识

5. ##### 扩展extend标识
   
   在官方extend标识只能在interface使用的基础上，我们进行扩展，使其能够在方法上使用

6. ##### 扩展可以生成拷贝的模式
   
   1. 针对所有可以转换成struct对struct的拷贝，我们在生成函数时，将struct to struct 装变为struct pointer to struct pointer 以减少内存的拷贝，同时，对不可复制类型做到了保护
   
   2. 扩展拷贝模式 struct pointer to struct
